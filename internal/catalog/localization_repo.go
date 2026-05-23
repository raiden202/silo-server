package catalog

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Silo-Server/silo-server/internal/models"
)

type MediaItemLocalizationRepository struct {
	pool *pgxpool.Pool
}

func NewMediaItemLocalizationRepository(pool *pgxpool.Pool) *MediaItemLocalizationRepository {
	return &MediaItemLocalizationRepository{pool: pool}
}

func (r *MediaItemLocalizationRepository) Upsert(ctx context.Context, loc *models.MediaItemLocalization) error {
	if loc == nil || loc.ContentID == "" || loc.Language == "" {
		return fmt.Errorf("invalid media item localization")
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO media_item_localizations (
			content_id, language, title, sort_title, overview, tagline,
			poster_path, poster_thumbhash, backdrop_path, backdrop_thumbhash, logo_path
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		ON CONFLICT (content_id, language) DO UPDATE SET
			title = EXCLUDED.title,
			sort_title = EXCLUDED.sort_title,
			overview = EXCLUDED.overview,
			tagline = EXCLUDED.tagline,
			poster_path = EXCLUDED.poster_path,
			poster_thumbhash = EXCLUDED.poster_thumbhash,
			backdrop_path = EXCLUDED.backdrop_path,
			backdrop_thumbhash = EXCLUDED.backdrop_thumbhash,
			logo_path = EXCLUDED.logo_path,
			updated_at = NOW()
	`, loc.ContentID, loc.Language, loc.Title, loc.SortTitle, loc.Overview, loc.Tagline,
		loc.PosterPath, loc.PosterThumbhash, loc.BackdropPath, loc.BackdropThumbhash, loc.LogoPath)
	if err != nil {
		return fmt.Errorf("upserting media item localization: %w", err)
	}
	return nil
}

func (r *MediaItemLocalizationRepository) Get(ctx context.Context, contentID, language string) (*models.MediaItemLocalization, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT content_id, language, title, sort_title, overview, tagline,
		       poster_path, poster_thumbhash, backdrop_path, backdrop_thumbhash, logo_path,
		       created_at, updated_at
		FROM media_item_localizations
		WHERE content_id = $1 AND language = $2
	`, contentID, language)
	return scanMediaItemLocalization(row)
}

func (r *MediaItemLocalizationRepository) GetByContentIDs(ctx context.Context, contentIDs []string, language string) (map[string]*models.MediaItemLocalization, error) {
	result := make(map[string]*models.MediaItemLocalization, len(contentIDs))
	if len(contentIDs) == 0 || language == "" {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT content_id, language, title, sort_title, overview, tagline,
		       poster_path, poster_thumbhash, backdrop_path, backdrop_thumbhash, logo_path,
		       created_at, updated_at
		FROM media_item_localizations
		WHERE language = $1 AND content_id = ANY($2)
	`, language, contentIDs)
	if err != nil {
		return nil, fmt.Errorf("querying media item localizations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		loc, scanErr := scanMediaItemLocalization(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[loc.ContentID] = loc
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating media item localizations: %w", err)
	}
	return result, nil
}

type SeasonLocalizationRepository struct {
	pool *pgxpool.Pool
}

func NewSeasonLocalizationRepository(pool *pgxpool.Pool) *SeasonLocalizationRepository {
	return &SeasonLocalizationRepository{pool: pool}
}

func (r *SeasonLocalizationRepository) Upsert(ctx context.Context, loc *models.SeasonLocalization) error {
	if loc == nil || loc.SeasonContentID == "" || loc.Language == "" {
		return fmt.Errorf("invalid season localization")
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO season_localizations (
			season_content_id, language, title, overview, poster_path, poster_thumbhash
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (season_content_id, language) DO UPDATE SET
			title = EXCLUDED.title,
			overview = EXCLUDED.overview,
			poster_path = EXCLUDED.poster_path,
			poster_thumbhash = EXCLUDED.poster_thumbhash,
			updated_at = NOW()
	`, loc.SeasonContentID, loc.Language, loc.Title, loc.Overview, loc.PosterPath, loc.PosterThumbhash)
	if err != nil {
		return fmt.Errorf("upserting season localization: %w", err)
	}
	return nil
}

func (r *SeasonLocalizationRepository) Get(ctx context.Context, seasonContentID, language string) (*models.SeasonLocalization, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT season_content_id, language, title, overview, poster_path, poster_thumbhash, created_at, updated_at
		FROM season_localizations
		WHERE season_content_id = $1 AND language = $2
	`, seasonContentID, language)
	return scanSeasonLocalization(row)
}

func (r *SeasonLocalizationRepository) GetBySeasonIDs(ctx context.Context, seasonIDs []string, language string) (map[string]*models.SeasonLocalization, error) {
	result := make(map[string]*models.SeasonLocalization, len(seasonIDs))
	if len(seasonIDs) == 0 || language == "" {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT season_content_id, language, title, overview, poster_path, poster_thumbhash, created_at, updated_at
		FROM season_localizations
		WHERE language = $1 AND season_content_id = ANY($2)
	`, language, seasonIDs)
	if err != nil {
		return nil, fmt.Errorf("querying season localizations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		loc, scanErr := scanSeasonLocalization(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[loc.SeasonContentID] = loc
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating season localizations: %w", err)
	}
	return result, nil
}

type EpisodeLocalizationRepository struct {
	pool *pgxpool.Pool
}

func NewEpisodeLocalizationRepository(pool *pgxpool.Pool) *EpisodeLocalizationRepository {
	return &EpisodeLocalizationRepository{pool: pool}
}

func (r *EpisodeLocalizationRepository) Upsert(ctx context.Context, loc *models.EpisodeLocalization) error {
	if loc == nil || loc.EpisodeContentID == "" || loc.Language == "" {
		return fmt.Errorf("invalid episode localization")
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO episode_localizations (episode_content_id, language, title, overview)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (episode_content_id, language) DO UPDATE SET
			title = EXCLUDED.title,
			overview = EXCLUDED.overview,
			updated_at = NOW()
	`, loc.EpisodeContentID, loc.Language, loc.Title, loc.Overview)
	if err != nil {
		return fmt.Errorf("upserting episode localization: %w", err)
	}
	return nil
}

func (r *EpisodeLocalizationRepository) Get(ctx context.Context, episodeContentID, language string) (*models.EpisodeLocalization, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT episode_content_id, language, title, overview, created_at, updated_at
		FROM episode_localizations
		WHERE episode_content_id = $1 AND language = $2
	`, episodeContentID, language)
	return scanEpisodeLocalization(row)
}

func (r *EpisodeLocalizationRepository) GetByEpisodeIDs(ctx context.Context, episodeIDs []string, language string) (map[string]*models.EpisodeLocalization, error) {
	result := make(map[string]*models.EpisodeLocalization, len(episodeIDs))
	if len(episodeIDs) == 0 || language == "" {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT episode_content_id, language, title, overview, created_at, updated_at
		FROM episode_localizations
		WHERE language = $1 AND episode_content_id = ANY($2)
	`, language, episodeIDs)
	if err != nil {
		return nil, fmt.Errorf("querying episode localizations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		loc, scanErr := scanEpisodeLocalization(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[loc.EpisodeContentID] = loc
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating episode localizations: %w", err)
	}
	return result, nil
}

func scanMediaItemLocalization(row pgx.Row) (*models.MediaItemLocalization, error) {
	var loc models.MediaItemLocalization
	if err := row.Scan(
		&loc.ContentID,
		&loc.Language,
		&loc.Title,
		&loc.SortTitle,
		&loc.Overview,
		&loc.Tagline,
		&loc.PosterPath,
		&loc.PosterThumbhash,
		&loc.BackdropPath,
		&loc.BackdropThumbhash,
		&loc.LogoPath,
		&loc.CreatedAt,
		&loc.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning media item localization: %w", err)
	}
	return &loc, nil
}

func scanSeasonLocalization(row pgx.Row) (*models.SeasonLocalization, error) {
	var loc models.SeasonLocalization
	if err := row.Scan(
		&loc.SeasonContentID,
		&loc.Language,
		&loc.Title,
		&loc.Overview,
		&loc.PosterPath,
		&loc.PosterThumbhash,
		&loc.CreatedAt,
		&loc.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning season localization: %w", err)
	}
	return &loc, nil
}

func scanEpisodeLocalization(row pgx.Row) (*models.EpisodeLocalization, error) {
	var loc models.EpisodeLocalization
	if err := row.Scan(
		&loc.EpisodeContentID,
		&loc.Language,
		&loc.Title,
		&loc.Overview,
		&loc.CreatedAt,
		&loc.UpdatedAt,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scanning episode localization: %w", err)
	}
	return &loc, nil
}
