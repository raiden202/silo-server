package metadata

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/models"
)

// StaleMediaIDRepository persists external IDs that 404 during metadata refresh.
type StaleMediaIDRepository struct {
	pool *pgxpool.Pool
}

// NewStaleMediaIDRepository creates a new StaleMediaIDRepository.
func NewStaleMediaIDRepository(pool *pgxpool.Pool) *StaleMediaIDRepository {
	return &StaleMediaIDRepository{pool: pool}
}

const staleMediaIDColumns = `content_id, provider, provider_id, first_seen_at, last_seen_at`

func scanStaleMediaIDs(rows pgx.Rows) ([]*models.StaleMediaID, error) {
	var ids []*models.StaleMediaID
	for rows.Next() {
		var id models.StaleMediaID
		if err := rows.Scan(
			&id.ContentID,
			&id.Provider,
			&id.ProviderID,
			&id.FirstSeenAt,
			&id.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scanning stale media ID row: %w", err)
		}
		ids = append(ids, &id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating stale media ID rows: %w", err)
	}
	if ids == nil {
		ids = []*models.StaleMediaID{}
	}
	return ids, nil
}

// Upsert inserts or updates a stale external ID record.
func (r *StaleMediaIDRepository) Upsert(ctx context.Context, contentID, provider, providerID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO stale_media_ids (content_id, provider, provider_id, first_seen_at, last_seen_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (content_id, provider) DO UPDATE
		SET provider_id = EXCLUDED.provider_id,
		    last_seen_at = NOW()
	`, contentID, provider, providerID)
	if err != nil {
		return fmt.Errorf("upserting stale media ID: %w", err)
	}
	return nil
}

// GetByContentID loads all stale external ID records for a single item.
func (r *StaleMediaIDRepository) GetByContentID(ctx context.Context, contentID string) ([]*models.StaleMediaID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+staleMediaIDColumns+`
		FROM stale_media_ids
		WHERE content_id = $1
		ORDER BY provider ASC
	`, contentID)
	if err != nil {
		return nil, fmt.Errorf("getting stale media IDs by content_id: %w", err)
	}
	defer rows.Close()
	return scanStaleMediaIDs(rows)
}

// DeleteByContentID removes all stale ID records for an item.
func (r *StaleMediaIDRepository) DeleteByContentID(ctx context.Context, contentID string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM stale_media_ids WHERE content_id = $1`, contentID)
	if err != nil {
		return fmt.Errorf("deleting stale media IDs: %w", err)
	}
	return nil
}

// ListAll returns all stale IDs ordered by most recent sighting.
func (r *StaleMediaIDRepository) ListAll(ctx context.Context) ([]*models.StaleMediaID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+staleMediaIDColumns+`
		FROM stale_media_ids
		ORDER BY last_seen_at DESC, content_id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing stale media IDs: %w", err)
	}
	defer rows.Close()
	return scanStaleMediaIDs(rows)
}
