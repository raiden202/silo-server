package recipes

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CollectionParams is the typed config for a collection section. Exactly one
// of LibraryCollectionID (admin-curated) or UserCollectionID (profile-scoped
// personal collection) must be set — the dispatcher picks the right backend
// path based on which field is populated.
type CollectionParams struct {
	LibraryCollectionID string `json:"library_collection_id,omitempty"`
	UserCollectionID    string `json:"user_collection_id,omitempty"`
}

type collectionRecipe struct{}

func (collectionRecipe) Type() string                   { return "collection" }
func (collectionRecipe) NewParams() any                 { return &CollectionParams{} }
func (collectionRecipe) DefaultCacheTTL() time.Duration { return 10 * time.Minute }
func (collectionRecipe) Resolve(rc ResolverContext) (ResolvedItems, error) {
	return delegateResolve("collection", rc)
}
func (collectionRecipe) Validate(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("collection: missing collection id")
	}
	var p CollectionParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if p.LibraryCollectionID == "" && p.UserCollectionID == "" {
		return errors.New("collection: library_collection_id or user_collection_id is required")
	}
	return nil
}
func (collectionRecipe) Definition() RecipeDefinition {
	traktVariants := []struct {
		preset, mediaType, keySuffix, label, icon, description string
	}{
		{"trending", "movie", "movies", "Trakt Trending Movies", "📈", "Show a synced Trakt trending movies collection."},
		{"trending", "tv", "shows", "Trakt Trending Shows", "📈", "Show a synced Trakt trending shows collection."},
		{"popular", "movie", "movies", "Trakt Popular Movies", "⭐", "Show a synced Trakt popular movies collection."},
		{"popular", "tv", "shows", "Trakt Popular Shows", "⭐", "Show a synced Trakt popular shows collection."},
		{"recommended", "movie", "movies", "Trakt Recommended Movies", "🎯", "Show a synced Trakt recommendations collection for a connected profile."},
		{"recommended", "tv", "shows", "Trakt Recommended Shows", "🎯", "Show a synced Trakt recommendations collection for a connected profile."},
	}

	presets := []GalleryPreset{
		{
			Key:              "collection_pick",
			DisplayName:      "Collection",
			Icon:             "📁",
			DescriptionShort: "Show items from a library or curated collection.",
			DefaultParams:    json.RawMessage(`{"library_collection_id":""}`),
		},
	}
	for _, v := range traktVariants {
		params := fmt.Sprintf(
			`{"library_collection_id":"","source_provider":"trakt","source_preset":%q,"media_type":%q}`,
			v.preset, v.mediaType,
		)
		presets = append(presets, GalleryPreset{
			Key:              fmt.Sprintf("trakt_%s_%s", v.preset, v.keySuffix),
			DisplayName:      v.label,
			Icon:             v.icon,
			DescriptionShort: v.description,
			DefaultParams:    json.RawMessage(params),
		})
	}

	return RecipeDefinition{
		Type:     "collection",
		Category: CategoryHandPicked,
		Presets:  presets,
	}
}

func init() {
	Register(collectionRecipe{})
}
