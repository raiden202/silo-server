package pgstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/userstore"
)

// --- Favorites ---

func (s *PostgresUserStore) AddFavorite(ctx context.Context, profileID, mediaItemID string) error {
	return s.AddFavoriteAt(ctx, profileID, mediaItemID, time.Now().UTC())
}

func (s *PostgresUserStore) AddFavoriteAt(ctx context.Context, profileID, mediaItemID string, addedAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_favorites (user_id, profile_id, media_item_id, added_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING`,
		s.userID, profileID, mediaItemID, addedAt.UTC(),
	)
	return err
}

func (s *PostgresUserStore) RemoveFavorite(ctx context.Context, profileID, mediaItemID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_favorites WHERE user_id = $1 AND profile_id = $2 AND media_item_id = $3`,
		s.userID, profileID, mediaItemID,
	)
	return err
}

func (s *PostgresUserStore) ListFavorites(ctx context.Context, profileID string, limit, offset int) ([]userstore.Favorite, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT profile_id, media_item_id, added_at FROM user_favorites
		 WHERE user_id = $1 AND profile_id = $2 ORDER BY added_at DESC LIMIT $3 OFFSET $4`,
		s.userID, profileID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing favorites: %w", err)
	}
	defer rows.Close()

	var favorites []userstore.Favorite
	for rows.Next() {
		var f userstore.Favorite
		var addedAt time.Time
		if err := rows.Scan(&f.ProfileID, &f.MediaItemID, &addedAt); err != nil {
			return nil, fmt.Errorf("scanning favorite row: %w", err)
		}
		f.AddedAt = timeToString(addedAt)
		favorites = append(favorites, f)
	}
	return favorites, rows.Err()
}

func (s *PostgresUserStore) IsFavorite(ctx context.Context, profileID, mediaItemID string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_favorites WHERE user_id = $1 AND profile_id = $2 AND media_item_id = $3`,
		s.userID, profileID, mediaItemID,
	).Scan(&count)
	return count > 0, err
}

func (s *PostgresUserStore) ListFavoritesByMediaItems(ctx context.Context, profileID string, mediaItemIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(mediaItemIDs))
	if len(mediaItemIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(mediaItemIDs))
	args := make([]any, 0, len(mediaItemIDs)+2)
	args = append(args, s.userID, profileID)
	for i, mediaItemID := range mediaItemIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, mediaItemID)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT media_item_id FROM user_favorites
		 WHERE user_id = $1 AND profile_id = $2 AND media_item_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("listing favorites by media items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaItemID string
		if err := rows.Scan(&mediaItemID); err != nil {
			return nil, fmt.Errorf("scanning favorite row: %w", err)
		}
		result[mediaItemID] = true
	}
	return result, rows.Err()
}

// --- Watchlist ---

func (s *PostgresUserStore) AddToWatchlist(ctx context.Context, profileID, mediaItemID string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO user_watchlist (user_id, profile_id, media_item_id, added_at)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT DO NOTHING`,
		s.userID, profileID, mediaItemID, nowUTC(),
	)
	return err
}

func (s *PostgresUserStore) RemoveFromWatchlist(ctx context.Context, profileID, mediaItemID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM user_watchlist WHERE user_id = $1 AND profile_id = $2 AND media_item_id = $3`,
		s.userID, profileID, mediaItemID,
	)
	return err
}

func (s *PostgresUserStore) ListWatchlist(ctx context.Context, profileID string, limit, offset int) ([]userstore.WatchlistEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT profile_id, media_item_id, added_at FROM user_watchlist
		 WHERE user_id = $1 AND profile_id = $2 ORDER BY added_at DESC LIMIT $3 OFFSET $4`,
		s.userID, profileID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing watchlist: %w", err)
	}
	defer rows.Close()

	var entries []userstore.WatchlistEntry
	for rows.Next() {
		var w userstore.WatchlistEntry
		var addedAt time.Time
		if err := rows.Scan(&w.ProfileID, &w.MediaItemID, &addedAt); err != nil {
			return nil, fmt.Errorf("scanning watchlist row: %w", err)
		}
		w.AddedAt = timeToString(addedAt)
		entries = append(entries, w)
	}
	return entries, rows.Err()
}

func (s *PostgresUserStore) InWatchlist(ctx context.Context, profileID, mediaItemID string) (bool, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM user_watchlist WHERE user_id = $1 AND profile_id = $2 AND media_item_id = $3`,
		s.userID, profileID, mediaItemID,
	).Scan(&count)
	return count > 0, err
}

func (s *PostgresUserStore) ListWatchlistByMediaItems(ctx context.Context, profileID string, mediaItemIDs []string) (map[string]bool, error) {
	result := make(map[string]bool, len(mediaItemIDs))
	if len(mediaItemIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(mediaItemIDs))
	args := make([]any, 0, len(mediaItemIDs)+2)
	args = append(args, s.userID, profileID)
	for i, mediaItemID := range mediaItemIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, mediaItemID)
	}

	rows, err := s.pool.Query(ctx,
		`SELECT media_item_id FROM user_watchlist
		 WHERE user_id = $1 AND profile_id = $2 AND media_item_id IN (`+strings.Join(placeholders, ",")+`)`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("listing watchlist by media items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mediaItemID string
		if err := rows.Scan(&mediaItemID); err != nil {
			return nil, fmt.Errorf("scanning watchlist row: %w", err)
		}
		result[mediaItemID] = true
	}
	return result, rows.Err()
}
