-- +goose Up
ALTER TABLE ebook_enrichment_state
    ADD COLUMN status text NOT NULL DEFAULT 'pending',
    ADD COLUMN priority integer NOT NULL DEFAULT 0,
    ADD COLUMN attempts integer NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at timestamptz NOT NULL DEFAULT now(),
    ADD COLUMN lease_until timestamptz,
    ADD COLUMN last_attempt_at timestamptz,
    ADD COLUMN completed_at timestamptz,
    ADD COLUMN outcome text,
    ADD COLUMN last_error_class text,
    ADD COLUMN last_error text,
    ADD CONSTRAINT ebook_enrichment_state_status_check
        CHECK (status IN ('pending', 'running')),
    ADD CONSTRAINT ebook_enrichment_state_attempts_check
        CHECK (attempts >= 0);

-- Rows already tracked by the former failure counter belong to the legacy
-- backlog. Preserve their history but keep them behind incremental work.
UPDATE ebook_enrichment_state
SET attempts = failures,
    priority = -100;

-- Snapshot the pre-migration library as durable low-priority backfill. Runtime
-- materialization assigns new discoveries priority 100, so this backlog cannot
-- occupy claim slots ahead of incremental or due refresh work.
INSERT INTO ebook_enrichment_state (
    content_id,
    status,
    priority,
    attempts,
    next_attempt_at,
    updated_at
)
SELECT mi.content_id, 'pending', -100, 0, now(), now()
FROM media_items mi
WHERE mi.type = 'ebook'
  AND NOT EXISTS (
      SELECT 1
      FROM manga_chapters mc
      WHERE mc.chapter_content_id = mi.content_id
  )
  AND mi.last_refreshed IS NULL
ON CONFLICT (content_id) DO NOTHING;

CREATE INDEX ebook_enrichment_state_claim_idx
    ON ebook_enrichment_state (priority DESC, next_attempt_at, updated_at)
    WHERE status IN ('pending', 'running');

-- +goose Down
DROP INDEX IF EXISTS ebook_enrichment_state_claim_idx;

-- The former state table only retained positive failure rows. Remove queue rows
-- created by this migration/runtime before restoring that representation.
DELETE FROM ebook_enrichment_state
WHERE failures = 0;

ALTER TABLE ebook_enrichment_state
    DROP CONSTRAINT IF EXISTS ebook_enrichment_state_attempts_check,
    DROP CONSTRAINT IF EXISTS ebook_enrichment_state_status_check,
    DROP COLUMN IF EXISTS last_error,
    DROP COLUMN IF EXISTS last_error_class,
    DROP COLUMN IF EXISTS outcome,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS last_attempt_at,
    DROP COLUMN IF EXISTS lease_until,
    DROP COLUMN IF EXISTS next_attempt_at,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS status;
