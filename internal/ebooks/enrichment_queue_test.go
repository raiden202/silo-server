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
		"priority + FLOOR(EXTRACT(EPOCH FROM (now() - next_attempt_at)) / 3600)::integer",
		"SET status = 'running'",
		"lease_until = now() + $2::interval",
		"claim_token = gen_random_uuid()::text",
		"attempts = attempts + 1",
		"RETURNING state.content_id, state.claim_token, state.attempts",
		"state.protected_fields",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("claim query missing %q:\n%s", fragment, claimEnrichmentJobsQuery)
		}
	}
}

func TestEnrichmentQueueCapturesEveryProviderWritablePendingField(t *testing.T) {
	query := strings.Join(strings.Fields(ebookProtectedFieldsSQL), " ")
	for _, field := range []string{
		"title",
		"year",
		"overview",
		"tagline",
		"content_rating",
		"runtime",
		"release_date",
		"genres",
		"studios",
		"authors",
		"poster_path",
		"backdrop_path",
		"logo_path",
	} {
		if !strings.Contains(query, "'"+field+"'") {
			t.Fatalf("protected-field capture missing %q:\n%s", field, ebookProtectedFieldsSQL)
		}
	}
}

func TestEnrichmentQueueChecksExactClaimTokenBeforePersistence(t *testing.T) {
	query := strings.Join(strings.Fields(checkEnrichmentClaimQuery), " ")
	for _, fragment := range []string{
		"SELECT EXISTS",
		"content_id = $1",
		"status = 'running'",
		"claim_token = $2",
		"lease_until > now()",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("claim ownership query missing %q:\n%s", fragment, checkEnrichmentClaimQuery)
		}
	}
}

func TestEnrichmentQueueEnqueueMakesPendingRowsDueAndDefersRunningRequeue(t *testing.T) {
	query := strings.Join(strings.Fields(enqueueEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"INSERT INTO ebook_enrichment_state",
		"FROM media_items mi",
		"ON CONFLICT (content_id) DO UPDATE SET",
		"WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.next_attempt_at",
		"ELSE now()",
		"requeue_requested = ebook_enrichment_state.requeue_requested OR ebook_enrichment_state.status = 'running'",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("enqueue query missing %q:\n%s", fragment, enqueueEnrichmentJobQuery)
		}
	}
	if strings.Contains(query, "lease_until =") || strings.Contains(query, "claim_token =") {
		t.Fatalf("enqueue must not mutate an active lease:\n%s", enqueueEnrichmentJobQuery)
	}
}

func TestEnrichmentQueueMigrationIsCrashSafeAndSeedsAllLegacyEbooks(t *testing.T) {
	body, err := os.ReadFile("../../migrations/sql/20260719090000_ebook_enrichment_jobs.sql")
	if err != nil {
		t.Fatalf("read enrichment queue migration: %v", err)
	}
	migration := strings.Join(strings.Fields(string(body)), " ")

	for _, fragment := range []string{
		"-- +goose NO TRANSACTION",
		"ADD COLUMN IF NOT EXISTS claim_token text",
		"ADD COLUMN IF NOT EXISTS requeue_requested boolean NOT NULL DEFAULT false",
		"ADD COLUMN IF NOT EXISTS protected_fields text[] NOT NULL DEFAULT '{}'::text[]",
		"INSERT INTO ebook_enrichment_state",
		"CASE WHEN mi.last_refreshed IS NULL THEN -100 ELSE 0 END",
		"GREATEST(mi.last_refreshed + interval '90 days', now())",
		"mi.type = 'ebook'",
		"NOT EXISTS",
		"manga_chapters",
		"ON CONFLICT (content_id) DO NOTHING",
		"NOT i.indisvalid",
		"DROP INDEX public.ebook_enrichment_state_claim_idx",
		"CREATE INDEX CONCURRENTLY IF NOT EXISTS ebook_enrichment_state_claim_idx",
		"DROP INDEX CONCURRENTLY IF EXISTS ebook_enrichment_state_claim_idx",
	} {
		if !strings.Contains(migration, fragment) {
			t.Fatalf("legacy backlog migration missing %q:\n%s", fragment, body)
		}
	}
	if strings.Contains(migration, "AND mi.last_refreshed IS NULL") {
		t.Fatalf("migration must seed refreshed ebooks too:\n%s", body)
	}
}

func TestEnrichmentQueueMigrationDownPreservesCurrentFailureHistory(t *testing.T) {
	body, err := os.ReadFile("../../migrations/sql/20260719090000_ebook_enrichment_jobs.sql")
	if err != nil {
		t.Fatalf("read enrichment queue migration: %v", err)
	}
	parts := strings.SplitN(strings.Join(strings.Fields(string(body)), " "), "-- +goose Down", 2)
	if len(parts) != 2 {
		t.Fatal("migration missing Down section")
	}
	down := parts[1]
	for _, fragment := range []string{
		"SET failures = attempts",
		"WHERE outcome = 'failed'",
		"DELETE FROM ebook_enrichment_state WHERE outcome IN ('success', 'no_match')",
		"IF EXISTS",
	} {
		if !strings.Contains(down, fragment) {
			t.Fatalf("down migration missing %q:\n%s", fragment, body)
		}
	}
	if strings.Contains(down, "WHERE failures = 0") {
		t.Fatalf("down migration must not erase post-migration failures based on the stale legacy counter:\n%s", body)
	}
}

func TestEnrichmentQueueTransitionsKeepDurableRowsAndReleaseLeases(t *testing.T) {
	complete := strings.Join(strings.Fields(completeEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"UPDATE ebook_enrichment_state",
		"status = 'pending'",
		"lease_until = NULL",
		"completed_at = now()",
		"ELSE now() + $3::interval",
		"outcome = $2",
		"attempts = 0",
		"WHEN requeue_requested THEN 100 ELSE 0 END",
		"requeue_requested = false",
		"AND claim_token = $4",
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
		"AND claim_token = $2",
	} {
		if !strings.Contains(release, fragment) {
			t.Fatalf("release query missing %q:\n%s", fragment, releaseEnrichmentJobQuery)
		}
	}

	failure := strings.Join(strings.Fields(failEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"WHEN requeue_requested THEN now()",
		"WHEN requeue_requested THEN 100 ELSE priority END",
		"AND claim_token = $5",
	} {
		if !strings.Contains(failure, fragment) {
			t.Fatalf("failure transition missing %q:\n%s", fragment, failEnrichmentJobQuery)
		}
	}

	discard := strings.Join(strings.Fields(discardEnrichmentJobQuery), " ")
	for _, fragment := range []string{
		"status = 'discarded'",
		"lease_until = NULL",
		"claim_token = NULL",
		"outcome = 'discarded'",
		"WHERE content_id = $1",
		"AND claim_token = $2",
	} {
		if !strings.Contains(discard, fragment) {
			t.Fatalf("discard transition missing %q:\n%s", fragment, discardEnrichmentJobQuery)
		}
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
