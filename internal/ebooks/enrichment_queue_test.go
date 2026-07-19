package ebooks

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestEnrichmentQueueClaimQueryUsesAtomicLeasedClaims(t *testing.T) {
	query := strings.Join(strings.Fields(claimEnrichmentJobsQuery), " ")

	for _, fragment := range []string{
		"WITH candidates AS",
		"FOR UPDATE SKIP LOCKED",
		"status = 'pending' OR (status = 'running' AND lease_until < now())",
		"next_attempt_at <= now()",
		"ORDER BY priority DESC, next_attempt_at, updated_at",
		"SET status = 'running'",
		"lease_until = now() + $2::interval",
		"attempts = attempts + 1",
		"RETURNING state.content_id, state.attempts",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("claim query missing %q:\n%s", fragment, claimEnrichmentJobsQuery)
		}
	}
}

func TestEnrichmentQueueMaterializesUnrefreshedEbooksRegardlessOfPoster(t *testing.T) {
	query := strings.Join(strings.Fields(materializeEnrichmentJobsQuery), " ")

	if !strings.Contains(query, "SELECT mi.content_id, 'pending', 100, now(), now()") {
		t.Fatalf("newly discovered ebooks must enter ahead of legacy backfill:\n%s", materializeEnrichmentJobsQuery)
	}
	if !strings.Contains(query, "mi.last_refreshed IS NULL") {
		t.Fatalf("materialization must use refresh state as its eligibility gate:\n%s", materializeEnrichmentJobsQuery)
	}
	if strings.Contains(query, "poster_path") {
		t.Fatalf("local or embedded covers must not make ebooks ineligible for metadata enrichment:\n%s", materializeEnrichmentJobsQuery)
	}
}

func TestEnrichmentQueueMigrationSeedsLegacyBacklogAtLowPriority(t *testing.T) {
	body, err := os.ReadFile("../../migrations/sql/20260719090000_ebook_enrichment_jobs.sql")
	if err != nil {
		t.Fatalf("read enrichment queue migration: %v", err)
	}
	migration := strings.Join(strings.Fields(string(body)), " ")

	for _, fragment := range []string{
		"INSERT INTO ebook_enrichment_state",
		"SELECT mi.content_id, 'pending', -100, 0, now(), now()",
		"mi.type = 'ebook'",
		"mi.last_refreshed IS NULL",
		"NOT EXISTS",
		"manga_chapters",
		"ON CONFLICT (content_id) DO NOTHING",
	} {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("legacy backlog migration missing %q:\n%s", fragment, body)
		}
	}
	if strings.Contains(migration, "poster_path") {
		t.Fatalf("legacy backlog must include ebooks with local or embedded covers:\n%s", body)
	}
}

func TestEnrichmentQueueTransitionsKeepDurableRowsAndReleaseLeases(t *testing.T) {
	complete := strings.Join(strings.Fields(completeEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"UPDATE ebook_enrichment_state",
		"status = 'pending'",
		"lease_until = NULL",
		"completed_at = now()",
		"next_attempt_at = now() + $3::interval",
		"outcome = $2",
		"attempts = 0",
		"priority = GREATEST(priority, 0)",
		"AND last_attempt_at = $4",
	} {
		if !strings.Contains(complete, fragment) {
			t.Fatalf("complete query missing %q:\n%s", fragment, completeEnrichmentJobQuery)
		}
	}
	if strings.Contains(strings.ToUpper(complete), "DELETE") {
		t.Fatalf("completion must retain durable queue state:\n%s", completeEnrichmentJobQuery)
	}

	release := strings.Join(strings.Fields(releaseEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"status = 'pending'",
		"lease_until = NULL",
		"attempts = GREATEST(attempts - 1, 0)",
		"WHERE content_id = $1",
		"AND status = 'running'",
		"AND last_attempt_at = $2",
	} {
		if !strings.Contains(release, fragment) {
			t.Fatalf("release query missing %q:\n%s", fragment, releaseEnrichmentJobQuery)
		}
	}

	failure := strings.Join(strings.Fields(failEnrichmentJobQuery), " ")
	if !strings.Contains(failure, "AND last_attempt_at = $5") {
		t.Fatalf("failure transition must reject stale leases:\n%s", failEnrichmentJobQuery)
	}
}

func TestEnrichmentRetryPolicy(t *testing.T) {
	t.Run("outcome refresh horizons", func(t *testing.T) {
		if got := enrichmentRefreshHorizon(EnrichmentOutcomeSuccess); got != 90*24*time.Hour {
			t.Fatalf("success refresh horizon = %s, want 90 days", got)
		}
		if got := enrichmentRefreshHorizon(EnrichmentOutcomeNoMatch); got != 30*24*time.Hour {
			t.Fatalf("no-match refresh horizon = %s, want 30 days", got)
		}
	})

	t.Run("transient failures use capped exponential backoff", func(t *testing.T) {
		if got := enrichmentRetryDelay(EnrichmentErrorTransient, 1, 0); got != 5*time.Minute {
			t.Fatalf("first transient retry = %s, want 5m", got)
		}
		if got := enrichmentRetryDelay(EnrichmentErrorTransient, 2, 0); got != 10*time.Minute {
			t.Fatalf("second transient retry = %s, want 10m", got)
		}
		if got := enrichmentRetryDelay(EnrichmentErrorTransient, 20, 0); got != 24*time.Hour {
			t.Fatalf("capped transient retry = %s, want 24h", got)
		}
	})

	t.Run("rate limits honor provider horizon with a 24 hour cap", func(t *testing.T) {
		if got := enrichmentRetryDelay(EnrichmentErrorRateLimited, 1, 45*time.Minute); got != 45*time.Minute {
			t.Fatalf("rate-limited retry = %s, want 45m", got)
		}
		if got := enrichmentRetryDelay(EnrichmentErrorRateLimited, 1, 72*time.Hour); got != 24*time.Hour {
			t.Fatalf("capped rate-limited retry = %s, want 24h", got)
		}
	})

	t.Run("permanent failures refresh after 30 days", func(t *testing.T) {
		if got := enrichmentRetryDelay(EnrichmentErrorPermanent, 1, 0); got != 30*24*time.Hour {
			t.Fatalf("permanent retry = %s, want 30 days", got)
		}
	})
}

func TestEnrichmentRetryPolicyClassifiesProviderErrors(t *testing.T) {
	limited, err := status.New(codes.ResourceExhausted, "provider quota exhausted").WithDetails(
		&errdetails.RetryInfo{RetryDelay: durationpb.New(45 * time.Minute)},
	)
	if err != nil {
		t.Fatalf("attach retry info: %v", err)
	}
	providerErrors := errors.Join(fmt.Errorf("openlibrary search: %w", limited.Err()))
	errorClass, retryAfter := classifyEnrichmentError(fmt.Errorf("no metadata obtained: %w", providerErrors))
	if errorClass != EnrichmentErrorRateLimited || retryAfter != 45*time.Minute {
		t.Fatalf("rate-limit classification = (%q, %s), want (rate_limited, 45m)", errorClass, retryAfter)
	}

	for _, code := range []codes.Code{
		codes.InvalidArgument,
		codes.NotFound,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.Unimplemented,
	} {
		errorClass, retryAfter = classifyEnrichmentError(status.Error(code, "deterministic provider failure"))
		if errorClass != EnrichmentErrorPermanent || retryAfter != 0 {
			t.Fatalf("%s classification = (%q, %s), want (permanent, 0)", code, errorClass, retryAfter)
		}
	}

	errorClass, retryAfter = classifyEnrichmentError(status.Error(codes.Unavailable, "provider down"))
	if errorClass != EnrichmentErrorTransient || retryAfter != 0 {
		t.Fatalf("unavailable classification = (%q, %s), want (transient, 0)", errorClass, retryAfter)
	}
}
