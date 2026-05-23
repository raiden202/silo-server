package usercollections

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ServerVisibleCollection is the per-user view of a personal collection the
// owner has opted into their own library Collections tab. Personal collections
// are private to their owner; this list never crosses user boundaries.
type ServerVisibleCollection struct {
	ID               string `json:"id"`
	CreatorProfileID string `json:"creator_profile_id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	CollectionType   string `json:"collection_type"`
	ItemCount        int    `json:"item_count"`
	PosterPath       string `json:"-"`
	PosterURL        string `json:"poster_url,omitempty"`
	PosterThumbhash  string `json:"poster_thumbhash,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// serverVisibleListLimit caps how many opt-in collections a single library tab
// will surface so a user with hundreds of published collections can't blow up
// the response.
const serverVisibleListLimit = 500

// ListServerVisibleByLibrary returns the current user's personal collections
// that have opted into their library Collections tab and whose library scope
// matches the requested library. A collection with no library_ids set in its
// query definition is treated as library-agnostic and therefore visible in
// every library tab. Other users' collections are never returned.
func ListServerVisibleByLibrary(ctx context.Context, pool *pgxpool.Pool, userID int, libraryID int) ([]ServerVisibleCollection, error) {
	rows, err := pool.Query(ctx,
		`SELECT id, creator_profile_id, name, description, collection_type, item_count,
		        poster_url, poster_thumbhash, created_at, updated_at
		 FROM user_personal_collections
		 WHERE user_id = $1
		   AND include_in_server_collections = TRUE
		   AND (
		     NOT (query_definition ? 'library_ids')
		     OR jsonb_array_length(COALESCE(query_definition->'library_ids', '[]'::jsonb)) = 0
		     OR query_definition->'library_ids' @> to_jsonb($2::int)
		   )
		 ORDER BY name ASC
		 LIMIT $3`,
		userID, libraryID, serverVisibleListLimit,
	)
	if err != nil {
		return nil, fmt.Errorf("listing server-visible user collections: %w", err)
	}
	defer rows.Close()

	var out []ServerVisibleCollection
	for rows.Next() {
		var c ServerVisibleCollection
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&c.ID, &c.CreatorProfileID, &c.Name, &c.Description, &c.CollectionType,
			&c.ItemCount, &c.PosterPath, &c.PosterThumbhash, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning server-visible user collection: %w", err)
		}
		c.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		c.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		out = append(out, c)
	}
	return out, rows.Err()
}
