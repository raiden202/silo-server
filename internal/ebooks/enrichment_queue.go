package ebooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/catalog"
)

const (
	defaultEnrichmentLease = 10 * time.Minute
	maxEnrichmentRetry     = 24 * time.Hour
	transientRetryBase     = 5 * time.Minute
	skippedRetryHorizon    = 15 * time.Minute
)

type EnrichmentOutcome string

const (
	EnrichmentOutcomeSuccess EnrichmentOutcome = "success"
	EnrichmentOutcomeNoMatch EnrichmentOutcome = "no_match"
	EnrichmentOutcomeSkipped EnrichmentOutcome = "skipped"
)

type EnrichmentErrorClass string

const (
	EnrichmentErrorTransient   EnrichmentErrorClass = "transient"
	EnrichmentErrorRateLimited EnrichmentErrorClass = "rate_limited"
	EnrichmentErrorPermanent   EnrichmentErrorClass = "permanent"
)

var ErrEnrichmentLeaseLost = errors.New("ebook enrichment lease lost")

type EnrichmentJob struct {
	ContentID       string
	Token           string
	Attempts        int
	LastAttemptAt   time.Time
	ProtectedFields []string
}

type EnrichmentQueue struct {
	pool *pgxpool.Pool
}

func NewEnrichmentQueue(pool *pgxpool.Pool) *EnrichmentQueue {
	return &EnrichmentQueue{pool: pool}
}

const ebookProtectedFieldsSQL = `
	ARRAY_REMOVE(ARRAY[
		CASE WHEN lower(trim(mi.status)) = 'pending' AND trim(COALESCE(mi.title, '')) <> '' THEN 'title' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND COALESCE(mi.year, 0) > 0 THEN 'year' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND trim(COALESCE(mi.overview, '')) <> '' THEN 'overview' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND trim(COALESCE(mi.tagline, '')) <> '' THEN 'tagline' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND trim(COALESCE(mi.content_rating, '')) <> '' THEN 'content_rating' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND COALESCE(mi.runtime, 0) > 0 THEN 'runtime' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND trim(COALESCE(mi.release_date::text, '')) <> '' THEN 'release_date' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND cardinality(COALESCE(mi.genres, '{}'::text[])) > 0 THEN 'genres' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND cardinality(COALESCE(mi.studios, '{}'::text[])) > 0 THEN 'studios' END,
		CASE WHEN lower(trim(mi.status)) = 'pending' AND EXISTS (
			SELECT 1 FROM item_people ip WHERE ip.content_id = mi.content_id AND ip.kind = 7
		) THEN 'authors' END,
		CASE WHEN trim(COALESCE(mi.poster_path, '')) <> ''
			AND lower(trim(mi.poster_path)) NOT LIKE 'http://%'
			AND lower(trim(mi.poster_path)) NOT LIKE 'https://%'
			AND lower(trim(mi.poster_path)) NOT LIKE 'ebook-metadata/ebooks/%'
			THEN 'poster_path' END,
		CASE WHEN trim(COALESCE(mi.backdrop_path, '')) <> ''
			AND lower(trim(mi.backdrop_path)) NOT LIKE 'http://%'
			AND lower(trim(mi.backdrop_path)) NOT LIKE 'https://%'
			AND lower(trim(mi.backdrop_path)) NOT LIKE 'ebook-metadata/ebooks/%'
			THEN 'backdrop_path' END,
		CASE WHEN trim(COALESCE(mi.logo_path, '')) <> ''
			AND lower(trim(mi.logo_path)) NOT LIKE 'http://%'
			AND lower(trim(mi.logo_path)) NOT LIKE 'https://%'
			AND lower(trim(mi.logo_path)) NOT LIKE 'ebook-metadata/ebooks/%'
			THEN 'logo_path' END
	]::text[], NULL)
`

const mergeEbookProtectedFieldsSQL = `
	ARRAY(
		SELECT DISTINCT field
		FROM unnest(
			ebook_enrichment_state.protected_fields ||
			ARRAY(
				SELECT candidate
				FROM unnest(EXCLUDED.protected_fields) AS candidate
				WHERE candidate IN ('poster_path', 'backdrop_path', 'logo_path')
			)
		) AS field
		ORDER BY field
	)
`

var enqueueEnrichmentJobQuery = `
	INSERT INTO ebook_enrichment_state (
		content_id, status, priority, next_attempt_at, protected_fields, updated_at
	)
	SELECT mi.content_id, 'pending', $2, now(), ` + ebookProtectedFieldsSQL + `, now()
	FROM media_items mi
	WHERE mi.content_id = $1
	  AND mi.type = 'ebook'
	  AND ` + catalog.MangaChapterExclusionWhere("mi") + `
	ON CONFLICT (content_id) DO UPDATE SET
		status = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.status
			ELSE 'pending'
		END,
		priority = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.priority
			ELSE GREATEST(ebook_enrichment_state.priority, EXCLUDED.priority)
		END,
		next_attempt_at = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.next_attempt_at
			ELSE now()
		END,
		requeue_requested = ebook_enrichment_state.requeue_requested OR ebook_enrichment_state.status = 'running',
		protected_fields = ` + mergeEbookProtectedFieldsSQL + `,
		updated_at = now()
`

func (q *EnrichmentQueue) Enqueue(ctx context.Context, contentID string, priority int) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	if strings.TrimSpace(contentID) == "" {
		return errors.New("ebook enrichment content id is required")
	}
	_, err := q.pool.Exec(ctx, enqueueEnrichmentJobQuery, contentID, priority)
	return err
}

var materializeEnrichmentJobsQuery = `
	INSERT INTO ebook_enrichment_state (
		content_id, status, priority, next_attempt_at, completed_at, protected_fields, updated_at
	)
	SELECT
		mi.content_id,
		'pending',
		CASE WHEN mi.last_refreshed IS NULL THEN 100 ELSE 0 END,
		CASE
			WHEN mi.last_refreshed IS NULL THEN now()
			ELSE GREATEST(mi.last_refreshed + interval '90 days', now())
		END,
		mi.last_refreshed,
		` + ebookProtectedFieldsSQL + `,
		now()
	FROM media_items mi
	WHERE mi.type = 'ebook'
	  AND ` + catalog.MangaChapterExclusionWhere("mi") + `
	ON CONFLICT (content_id) DO UPDATE SET
		status = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.status
			WHEN ebook_enrichment_state.status = 'discarded' THEN 'pending'
			ELSE ebook_enrichment_state.status
		END,
		priority = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.priority
			WHEN ebook_enrichment_state.status = 'discarded' THEN EXCLUDED.priority
			WHEN EXCLUDED.completed_at IS NULL AND ebook_enrichment_state.completed_at IS NOT NULL THEN 100
			ELSE ebook_enrichment_state.priority
		END,
		next_attempt_at = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.next_attempt_at
			WHEN ebook_enrichment_state.status = 'discarded' THEN EXCLUDED.next_attempt_at
			WHEN EXCLUDED.completed_at IS NULL AND ebook_enrichment_state.completed_at IS NOT NULL THEN now()
			ELSE ebook_enrichment_state.next_attempt_at
		END,
		completed_at = CASE
			WHEN ebook_enrichment_state.status = 'running' THEN ebook_enrichment_state.completed_at
			WHEN ebook_enrichment_state.status = 'discarded' THEN EXCLUDED.completed_at
			WHEN EXCLUDED.completed_at IS NULL AND ebook_enrichment_state.completed_at IS NOT NULL THEN NULL
			ELSE ebook_enrichment_state.completed_at
		END,
		outcome = CASE
			WHEN ebook_enrichment_state.status = 'discarded'
				OR (EXCLUDED.completed_at IS NULL AND ebook_enrichment_state.completed_at IS NOT NULL)
			THEN NULL
			ELSE ebook_enrichment_state.outcome
		END,
		protected_fields = ` + mergeEbookProtectedFieldsSQL + `,
		updated_at = now()
`

func (q *EnrichmentQueue) MaterializeCandidates(ctx context.Context) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	_, err := q.pool.Exec(ctx, materializeEnrichmentJobsQuery)
	return err
}

var claimEnrichmentJobsQuery = `
	WITH candidates AS (
		SELECT content_id
		FROM ebook_enrichment_state
		WHERE next_attempt_at <= now()
		  AND (status = 'pending' OR (status = 'running' AND lease_until < now()))
		ORDER BY
			(priority + FLOOR(EXTRACT(EPOCH FROM (now() - next_attempt_at)) / 3600)::integer) DESC,
			priority DESC,
			next_attempt_at,
			updated_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	)
	UPDATE ebook_enrichment_state state
	SET status = 'running',
		lease_until = now() + $2::interval,
		claim_token = gen_random_uuid()::text,
		last_attempt_at = now(),
		attempts = attempts + 1,
		updated_at = now()
	FROM candidates
	WHERE state.content_id = candidates.content_id
	RETURNING state.content_id, state.claim_token, state.attempts, state.last_attempt_at, state.protected_fields
`

func (q *EnrichmentQueue) ClaimBatch(ctx context.Context, limit int, leaseDuration time.Duration) ([]EnrichmentJob, error) {
	if q == nil || q.pool == nil {
		return nil, errors.New("ebook enrichment queue is not configured")
	}
	if limit <= 0 {
		return nil, nil
	}
	if leaseDuration <= 0 {
		leaseDuration = defaultEnrichmentLease
	}

	rows, err := q.pool.Query(ctx, claimEnrichmentJobsQuery, limit, postgresInterval(leaseDuration))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]EnrichmentJob, 0, limit)
	for rows.Next() {
		var job EnrichmentJob
		if err := rows.Scan(&job.ContentID, &job.Token, &job.Attempts, &job.LastAttemptAt, &job.ProtectedFields); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return jobs, nil
}

var completeEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'pending',
		lease_until = NULL,
		claim_token = NULL,
		completed_at = now(),
		next_attempt_at = CASE
			WHEN requeue_requested THEN now()
			ELSE now() + $3::interval
		END,
		outcome = $2,
		attempts = 0,
		priority = CASE WHEN requeue_requested THEN 100 ELSE 0 END,
		requeue_requested = false,
		last_error_class = NULL,
		last_error = NULL,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND claim_token = $4
`

func (q *EnrichmentQueue) Complete(
	ctx context.Context,
	job EnrichmentJob,
	outcome EnrichmentOutcome,
	refreshAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	if job.ContentID == "" || job.Token == "" {
		return ErrEnrichmentLeaseLost
	}
	if refreshAfter <= 0 {
		refreshAfter = enrichmentRefreshHorizon(outcome)
	}
	if refreshAfter <= 0 {
		return fmt.Errorf("unsupported ebook enrichment outcome %q", outcome)
	}

	tag, err := q.pool.Exec(
		ctx,
		completeEnrichmentJobQuery,
		job.ContentID,
		string(outcome),
		postgresInterval(refreshAfter),
		job.Token,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEnrichmentLeaseLost
	}
	return nil
}

var failEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'pending',
		lease_until = NULL,
		claim_token = NULL,
		next_attempt_at = CASE
			WHEN requeue_requested THEN now()
			ELSE now() + $4::interval
		END,
		priority = CASE WHEN requeue_requested THEN 100 ELSE priority END,
		requeue_requested = false,
		outcome = 'failed',
		last_error_class = $2,
		last_error = $3,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND claim_token = $5
`

func (q *EnrichmentQueue) Fail(
	ctx context.Context,
	job EnrichmentJob,
	errorClass EnrichmentErrorClass,
	message string,
	retryAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	if job.ContentID == "" || job.Token == "" {
		return ErrEnrichmentLeaseLost
	}
	delay := enrichmentRetryDelay(errorClass, job.Attempts, retryAfter)
	tag, err := q.pool.Exec(
		ctx,
		failEnrichmentJobQuery,
		job.ContentID,
		string(errorClass),
		message,
		postgresInterval(delay),
		job.Token,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEnrichmentLeaseLost
	}
	return nil
}

var releaseEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'pending',
		lease_until = NULL,
		claim_token = NULL,
		attempts = GREATEST(attempts - 1, 0),
		next_attempt_at = CASE WHEN requeue_requested THEN now() ELSE next_attempt_at END,
		priority = CASE WHEN requeue_requested THEN 100 ELSE priority END,
		requeue_requested = false,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND claim_token = $2
`

func (q *EnrichmentQueue) Release(ctx context.Context, job EnrichmentJob) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	if job.ContentID == "" || job.Token == "" {
		return ErrEnrichmentLeaseLost
	}
	tag, err := q.pool.Exec(ctx, releaseEnrichmentJobQuery, job.ContentID, job.Token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEnrichmentLeaseLost
	}
	return nil
}

var discardEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'discarded',
		lease_until = NULL,
		claim_token = NULL,
		completed_at = now(),
		outcome = 'discarded',
		attempts = 0,
		priority = 0,
		requeue_requested = false,
		last_error_class = NULL,
		last_error = NULL,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND claim_token = $2
`

func (q *EnrichmentQueue) Discard(ctx context.Context, job EnrichmentJob) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	if job.ContentID == "" || job.Token == "" {
		return ErrEnrichmentLeaseLost
	}
	tag, err := q.pool.Exec(ctx, discardEnrichmentJobQuery, job.ContentID, job.Token)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrEnrichmentLeaseLost
	}
	return nil
}

func enrichmentRefreshHorizon(outcome EnrichmentOutcome) time.Duration {
	switch outcome {
	case EnrichmentOutcomeSuccess:
		return 90 * 24 * time.Hour
	case EnrichmentOutcomeNoMatch:
		return 30 * 24 * time.Hour
	case EnrichmentOutcomeSkipped:
		return skippedRetryHorizon
	default:
		return 0
	}
}

func enrichmentRetryDelay(errorClass EnrichmentErrorClass, attempts int, retryAfter time.Duration) time.Duration {
	switch errorClass {
	case EnrichmentErrorRateLimited:
		if retryAfter <= 0 {
			retryAfter = transientRetryDelay(attempts)
		}
		return min(retryAfter, maxEnrichmentRetry)
	case EnrichmentErrorPermanent:
		return 30 * 24 * time.Hour
	default:
		return transientRetryDelay(attempts)
	}
}

func transientRetryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := transientRetryBase
	for i := 1; i < attempts; i++ {
		if delay >= maxEnrichmentRetry/2 {
			return maxEnrichmentRetry
		}
		delay *= 2
	}
	return min(delay, maxEnrichmentRetry)
}

func postgresInterval(duration time.Duration) string {
	return fmt.Sprintf("%d microseconds", duration.Microseconds())
}
