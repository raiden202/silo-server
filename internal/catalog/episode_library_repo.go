package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EpisodeLibraryRepository provides maintenance operations for episode_libraries.
type EpisodeLibraryRepository struct {
	pool *pgxpool.Pool
}

// NewEpisodeLibraryRepository creates a new EpisodeLibraryRepository backed by the given pool.
func NewEpisodeLibraryRepository(pool *pgxpool.Pool) *EpisodeLibraryRepository {
	return &EpisodeLibraryRepository{pool: pool}
}

// ReconcileFolderMembership removes episode memberships for episodes that no longer
// have any present files in the given folder.
func (r *EpisodeLibraryRepository) ReconcileFolderMembership(ctx context.Context, folderID int) (int, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM episode_libraries el
		WHERE el.media_folder_id = $1
		  AND NOT EXISTS (
			SELECT 1
			FROM media_files mf
			WHERE mf.media_folder_id = el.media_folder_id
			  AND mf.episode_id = el.episode_id
			  AND mf.missing_since IS NULL
		  )
	`, folderID)
	if err != nil {
		return 0, fmt.Errorf("reconciling episode library membership: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
