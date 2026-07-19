package ebooks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	ContentID     string
	Attempts      int
	LastAttemptAt time.Time
}

type EnrichmentQueue struct {
	pool *pgxpool.Pool

	claimsMu sync.Mutex
	claims   map[string]EnrichmentJob
}

func NewEnrichmentQueue(pool *pgxpool.Pool) *EnrichmentQueue {
	return &EnrichmentQueue{pool: pool}
}

var enqueueEnrichmentJobQuery = `
	INSERT INTO ebook_enrichment_state (
		content_id, status, priority, next_attempt_at, updated_at
	)
	VALUES ($1, 'pending', $2, now(), now())
	ON CONFLICT (content_id) DO UPDATE SET
		priority = GREATEST(ebook_enrichment_state.priority, EXCLUDED.priority),
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
		content_id, status, priority, next_attempt_at, updated_at
	)
	SELECT mi.content_id, 'pending', 100, now(), now()
	FROM media_items mi
	WHERE mi.type = 'ebook'
	  AND ` + catalog.MangaChapterExclusionWhere("mi") + `
	  AND mi.last_refreshed IS NULL
	ON CONFLICT (content_id) DO NOTHING
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
		ORDER BY priority DESC, next_attempt_at, updated_at
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	)
	UPDATE ebook_enrichment_state state
	SET status = 'running',
		lease_until = now() + $2::interval,
		last_attempt_at = now(),
		attempts = attempts + 1,
		updated_at = now()
	FROM candidates
	WHERE state.content_id = candidates.content_id
	RETURNING state.content_id, state.attempts, state.last_attempt_at
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
		if err := rows.Scan(&job.ContentID, &job.Attempts, &job.LastAttemptAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
		q.rememberClaim(job)
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
		completed_at = now(),
		next_attempt_at = now() + $3::interval,
		outcome = $2,
		attempts = 0,
		priority = GREATEST(priority, 0),
		last_error_class = NULL,
		last_error = NULL,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND last_attempt_at = $4
`

func (q *EnrichmentQueue) Complete(
	ctx context.Context,
	contentID string,
	outcome EnrichmentOutcome,
	refreshAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	job, ok := q.claimedJob(contentID)
	if !ok {
		return ErrEnrichmentLeaseLost
	}
	return q.CompleteClaim(ctx, job, outcome, refreshAfter)
}

func (q *EnrichmentQueue) CompleteClaim(
	ctx context.Context,
	job EnrichmentJob,
	outcome EnrichmentOutcome,
	refreshAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
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
		job.LastAttemptAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		q.forgetClaim(job)
		return ErrEnrichmentLeaseLost
	}
	q.forgetClaim(job)
	return nil
}

var failEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'pending',
		lease_until = NULL,
		next_attempt_at = now() + $4::interval,
		outcome = 'failed',
		last_error_class = $2,
		last_error = $3,
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND last_attempt_at = $5
`

func (q *EnrichmentQueue) Fail(
	ctx context.Context,
	contentID string,
	errorClass EnrichmentErrorClass,
	message string,
	retryAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	job, ok := q.claimedJob(contentID)
	if !ok {
		return ErrEnrichmentLeaseLost
	}
	return q.FailClaim(ctx, job, errorClass, message, retryAfter)
}

func (q *EnrichmentQueue) FailClaim(
	ctx context.Context,
	job EnrichmentJob,
	errorClass EnrichmentErrorClass,
	message string,
	retryAfter time.Duration,
) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	delay := enrichmentRetryDelay(errorClass, job.Attempts, retryAfter)
	tag, err := q.pool.Exec(
		ctx,
		failEnrichmentJobQuery,
		job.ContentID,
		string(errorClass),
		message,
		postgresInterval(delay),
		job.LastAttemptAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		q.forgetClaim(job)
		return ErrEnrichmentLeaseLost
	}
	q.forgetClaim(job)
	return nil
}

var releaseEnrichmentJobQuery = `
	UPDATE ebook_enrichment_state
	SET status = 'pending',
		lease_until = NULL,
		attempts = GREATEST(attempts - 1, 0),
		updated_at = now()
	WHERE content_id = $1
	  AND status = 'running'
	  AND last_attempt_at = $2
`

func (q *EnrichmentQueue) Release(ctx context.Context, contentID string) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	job, ok := q.claimedJob(contentID)
	if !ok {
		return ErrEnrichmentLeaseLost
	}
	return q.ReleaseClaim(ctx, job)
}

func (q *EnrichmentQueue) ReleaseClaim(ctx context.Context, job EnrichmentJob) error {
	if q == nil || q.pool == nil {
		return errors.New("ebook enrichment queue is not configured")
	}
	tag, err := q.pool.Exec(ctx, releaseEnrichmentJobQuery, job.ContentID, job.LastAttemptAt)
	if err != nil {
		return err
	}
	q.forgetClaim(job)
	if tag.RowsAffected() == 0 {
		return ErrEnrichmentLeaseLost
	}
	return nil
}

func (q *EnrichmentQueue) rememberClaim(job EnrichmentJob) {
	q.claimsMu.Lock()
	defer q.claimsMu.Unlock()
	if q.claims == nil {
		q.claims = make(map[string]EnrichmentJob)
	}
	q.claims[job.ContentID] = job
}

func (q *EnrichmentQueue) claimedJob(contentID string) (EnrichmentJob, bool) {
	q.claimsMu.Lock()
	defer q.claimsMu.Unlock()
	job, ok := q.claims[contentID]
	return job, ok
}

func (q *EnrichmentQueue) forgetClaim(job EnrichmentJob) {
	q.claimsMu.Lock()
	defer q.claimsMu.Unlock()
	current, ok := q.claims[job.ContentID]
	if ok && current.LastAttemptAt.Equal(job.LastAttemptAt) {
		delete(q.claims, job.ContentID)
	}
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
