package download

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const downloadColumns = `id, user_id, media_file_id, content_id, episode_id, batch_id,
	kind, status, file_size, bytes_sent, error_message,
	created_at, updated_at, completed_at`

// Repository provides CRUD operations for the downloads table.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository backed by the given pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func scanDownload(row pgx.Row) (*Download, error) {
	var d Download
	var episodeID, batchID *string
	err := row.Scan(
		&d.ID, &d.UserID, &d.MediaFileID, &d.ContentID, &episodeID, &batchID,
		&d.Kind, &d.Status, &d.FileSize, &d.BytesSent, &d.ErrorMessage,
		&d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scanning download: %w", err)
	}
	if episodeID != nil {
		d.EpisodeID = *episodeID
	}
	if batchID != nil {
		d.BatchID = *batchID
	}
	return &d, nil
}

func scanDownloads(rows pgx.Rows) ([]*Download, error) {
	var downloads []*Download
	for rows.Next() {
		var d Download
		var episodeID, batchID *string
		err := rows.Scan(
			&d.ID, &d.UserID, &d.MediaFileID, &d.ContentID, &episodeID, &batchID,
			&d.Kind, &d.Status, &d.FileSize, &d.BytesSent, &d.ErrorMessage,
			&d.CreatedAt, &d.UpdatedAt, &d.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning download row: %w", err)
		}
		if episodeID != nil {
			d.EpisodeID = *episodeID
		}
		if batchID != nil {
			d.BatchID = *batchID
		}
		downloads = append(downloads, &d)
	}
	return downloads, rows.Err()
}

// Create inserts a new download record.
func (r *Repository) Create(ctx context.Context, d *Download) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO downloads (id, user_id, media_file_id, content_id, episode_id, batch_id,
			kind, status, file_size, bytes_sent, error_message, created_at, updated_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		d.ID, d.UserID, d.MediaFileID, d.ContentID,
		nilIfEmpty(d.EpisodeID), nilIfEmpty(d.BatchID),
		d.Kind, d.Status, d.FileSize, d.BytesSent, d.ErrorMessage,
		d.CreatedAt, d.UpdatedAt, d.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting download: %w", err)
	}
	return nil
}

// CreateBatch inserts multiple download records in a single transaction.
func (r *Repository) CreateBatch(ctx context.Context, downloads []*Download) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning batch insert: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, d := range downloads {
		_, err := tx.Exec(ctx,
			`INSERT INTO downloads (id, user_id, media_file_id, content_id, episode_id, batch_id,
				kind, status, file_size, bytes_sent, error_message, created_at, updated_at, completed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			d.ID, d.UserID, d.MediaFileID, d.ContentID,
			nilIfEmpty(d.EpisodeID), nilIfEmpty(d.BatchID),
			d.Kind, d.Status, d.FileSize, d.BytesSent, d.ErrorMessage,
			d.CreatedAt, d.UpdatedAt, d.CompletedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting batch download: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing batch insert: %w", err)
	}
	return nil
}

// GetByID retrieves a download by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*Download, error) {
	query := `SELECT ` + downloadColumns + ` FROM downloads WHERE id = $1`
	return scanDownload(r.pool.QueryRow(ctx, query, id))
}

// ListByUser returns all downloads for a user, most recent first.
func (r *Repository) ListByUser(ctx context.Context, userID int) ([]*Download, error) {
	query := `SELECT ` + downloadColumns + ` FROM downloads
		WHERE user_id = $1
		ORDER BY created_at DESC`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing downloads for user: %w", err)
	}
	defer rows.Close()
	result, err := scanDownloads(rows)
	if err != nil {
		return nil, fmt.Errorf("scanning download rows: %w", err)
	}
	return result, nil
}

// CountActiveByUser returns the number of active (queued or downloading) downloads for a user.
func (r *Repository) CountActiveByUser(ctx context.Context, userID int) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM downloads WHERE user_id = $1 AND status IN ('queued', 'downloading')`,
		userID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting active downloads: %w", err)
	}
	return count, nil
}

// CountByUserSince returns the number of successful downloads created since the given time.
// Cancelled and failed downloads are excluded so transient failures don't consume quota.
func (r *Repository) CountByUserSince(ctx context.Context, userID int, since time.Time) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM downloads WHERE user_id = $1 AND created_at >= $2 AND status NOT IN ('cancelled', 'failed')`,
		userID, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting downloads in period: %w", err)
	}
	return count, nil
}

// UpdateStatus sets the status and optionally the bytes_sent and completed_at fields.
func (r *Repository) UpdateStatus(ctx context.Context, id, status string, bytesSent int64, completedAt *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE downloads SET status = $1, bytes_sent = $2, completed_at = $3, updated_at = now() WHERE id = $4`,
		status, bytesSent, completedAt, id,
	)
	if err != nil {
		return fmt.Errorf("updating download status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TransitionStatus atomically transitions a download from expectedStatus to newStatus.
// Returns ErrStatusConflict if the row is not in expectedStatus (another request
// already transitioned it).
func (r *Repository) TransitionStatus(ctx context.Context, id, expectedStatus, newStatus string, bytesSent int64, completedAt *time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE downloads SET status = $1, bytes_sent = $2, completed_at = $3, updated_at = now()
		WHERE id = $4 AND status = $5`,
		newStatus, bytesSent, completedAt, id, expectedStatus,
	)
	if err != nil {
		return fmt.Errorf("transitioning download status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStatusConflict
	}
	return nil
}

// Delete removes a download record. Returns ErrNotFound if the row doesn't exist
// or doesn't belong to the given user.
func (r *Repository) Delete(ctx context.Context, id string, userID int) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM downloads WHERE id = $1 AND user_id = $2`, id, userID,
	)
	if err != nil {
		return fmt.Errorf("deleting download: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CancelByID sets a download to cancelled if it is still queued or downloading.
func (r *Repository) CancelByID(ctx context.Context, id string, userID int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE downloads SET status = 'cancelled', updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status IN ('queued', 'downloading')`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("cancelling download: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
