package pgstore

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type profileLibraryQuerier interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func listProfileAllowedLibraries(ctx context.Context, q profileLibraryQuerier, userID int, profileID string) ([]int, error) {
	rows, err := q.Query(ctx,
		`SELECT library_id
		FROM user_profile_allowed_libraries
		WHERE user_id = $1 AND profile_id = $2
		ORDER BY library_id ASC`,
		userID,
		profileID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing allowed libraries for profile %s: %w", profileID, err)
	}
	defer rows.Close()

	var libraryIDs []int
	for rows.Next() {
		var libraryID int
		if err := rows.Scan(&libraryID); err != nil {
			return nil, fmt.Errorf("scanning allowed library for profile %s: %w", profileID, err)
		}
		libraryIDs = append(libraryIDs, libraryID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating allowed libraries for profile %s: %w", profileID, err)
	}
	return libraryIDs, nil
}

func replaceProfileAllowedLibraries(ctx context.Context, q profileLibraryQuerier, userID int, profileID string, libraryIDs []int) error {
	if _, err := q.Exec(ctx,
		"DELETE FROM user_profile_allowed_libraries WHERE user_id = $1 AND profile_id = $2",
		userID,
		profileID,
	); err != nil {
		return fmt.Errorf("clearing allowed libraries for profile %s: %w", profileID, err)
	}
	for _, libraryID := range libraryIDs {
		if _, err := q.Exec(ctx,
			"INSERT INTO user_profile_allowed_libraries (user_id, profile_id, library_id) VALUES ($1, $2, $3)",
			userID,
			profileID,
			libraryID,
		); err != nil {
			return fmt.Errorf("inserting allowed library %d for profile %s: %w", libraryID, profileID, err)
		}
	}
	return nil
}
