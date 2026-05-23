package metadata

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/models"
)

// SkippedRootRepository persists media roots that were skipped during scans.
type SkippedRootRepository struct {
	pool *pgxpool.Pool
}

// NewSkippedRootRepository creates a new SkippedRootRepository.
func NewSkippedRootRepository(pool *pgxpool.Pool) *SkippedRootRepository {
	return &SkippedRootRepository{pool: pool}
}

const skippedRootColumns = `media_folder_id, root_path, reason, sample_file_path, file_count, first_seen_at, last_seen_at`

func scanSkippedRoot(row pgx.Row) (*models.SkippedMediaRoot, error) {
	var root models.SkippedMediaRoot
	if err := row.Scan(
		&root.MediaFolderID,
		&root.RootPath,
		&root.Reason,
		&root.SampleFilePath,
		&root.FileCount,
		&root.FirstSeenAt,
		&root.LastSeenAt,
	); err != nil {
		return nil, fmt.Errorf("scanning skipped media root: %w", err)
	}
	return &root, nil
}

func scanSkippedRoots(rows pgx.Rows) ([]*models.SkippedMediaRoot, error) {
	var roots []*models.SkippedMediaRoot
	for rows.Next() {
		var root models.SkippedMediaRoot
		if err := rows.Scan(
			&root.MediaFolderID,
			&root.RootPath,
			&root.Reason,
			&root.SampleFilePath,
			&root.FileCount,
			&root.FirstSeenAt,
			&root.LastSeenAt,
		); err != nil {
			return nil, fmt.Errorf("scanning skipped media root row: %w", err)
		}
		roots = append(roots, &root)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating skipped media root rows: %w", err)
	}
	if roots == nil {
		roots = []*models.SkippedMediaRoot{}
	}
	return roots, nil
}

// Upsert inserts or updates a skipped media root keyed by folder and root path.
func (r *SkippedRootRepository) Upsert(ctx context.Context, root models.SkippedMediaRoot) error {
	query := `
		INSERT INTO skipped_media_roots (
			media_folder_id, root_path, reason, sample_file_path, file_count, first_seen_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (media_folder_id, root_path) DO UPDATE
		SET reason = EXCLUDED.reason,
			sample_file_path = EXCLUDED.sample_file_path,
			file_count = EXCLUDED.file_count,
			last_seen_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query,
		root.MediaFolderID,
		root.RootPath,
		root.Reason,
		root.SampleFilePath,
		root.FileCount,
	)
	if err != nil {
		return fmt.Errorf("upserting skipped media root: %w", err)
	}
	return nil
}

// Delete removes one skipped media root.
func (r *SkippedRootRepository) Delete(ctx context.Context, folderID int, rootPath string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM skipped_media_roots WHERE media_folder_id = $1 AND root_path = $2`,
		folderID, rootPath,
	)
	if err != nil {
		return fmt.Errorf("deleting skipped media root: %w", err)
	}
	return nil
}

// DeleteMissingInScope removes skipped roots under scopePath that were not seen.
func (r *SkippedRootRepository) DeleteMissingInScope(ctx context.Context, folderID int, scopePath string, seenRoots []string) error {
	if seenRoots == nil {
		seenRoots = []string{}
	}

	scopePath = filepath.Clean(scopePath)

	_, err := r.pool.Exec(ctx, `
		DELETE FROM skipped_media_roots
		WHERE media_folder_id = $1
		  AND (root_path = $2 OR strpos(root_path, $2 || '/') = 1)
		  AND NOT (root_path = ANY($3))
	`, folderID, scopePath, seenRoots)
	if err != nil {
		return fmt.Errorf("deleting missing skipped media roots in scope: %w", err)
	}
	return nil
}

// ListByFolder returns skipped roots for a folder ordered by most recent sighting.
func (r *SkippedRootRepository) ListByFolder(ctx context.Context, folderID int) ([]*models.SkippedMediaRoot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+skippedRootColumns+`
		FROM skipped_media_roots
		WHERE media_folder_id = $1
		ORDER BY last_seen_at DESC, root_path ASC
	`, folderID)
	if err != nil {
		return nil, fmt.Errorf("listing skipped media roots by folder: %w", err)
	}
	defer rows.Close()
	return scanSkippedRoots(rows)
}

// ListAll returns all skipped roots ordered by most recent sighting.
func (r *SkippedRootRepository) ListAll(ctx context.Context) ([]*models.SkippedMediaRoot, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+skippedRootColumns+`
		FROM skipped_media_roots
		ORDER BY last_seen_at DESC, media_folder_id ASC, root_path ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing skipped media roots: %w", err)
	}
	defer rows.Close()
	return scanSkippedRoots(rows)
}
