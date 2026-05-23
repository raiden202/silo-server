package models

import "time"

// MediaItemProviderID represents a durable external ID attached to a media
// item. These rows store provider-specific identifiers beyond the canonical
// tmdb/imdb/tvdb columns on media_items.
type MediaItemProviderID struct {
	ContentID  string
	ItemType   string
	Provider   string
	ProviderID string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
