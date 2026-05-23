package recipes

import (
	"encoding/json"
	"time"
)

// libStapleParams is the (empty) param shape for parameter-free library staples.
type libStapleParams struct{}

// libStaple wraps a delegated resolver func with Recipe metadata.
type libStaple struct {
	typ         string
	displayName string
	icon        string
	descShort   string
	cacheTTL    time.Duration
}

func (l *libStaple) Type() string                     { return l.typ }
func (l *libStaple) NewParams() any                   { return &libStapleParams{} }
func (l *libStaple) Validate(_ json.RawMessage) error { return nil } // any JSON is acceptable; missing fields ignored
func (l *libStaple) DefaultCacheTTL() time.Duration   { return l.cacheTTL }

func (l *libStaple) Definition() RecipeDefinition {
	return RecipeDefinition{
		Type:     l.typ,
		Category: CategoryLibraryStaples,
		Presets: []GalleryPreset{
			{
				Key:              l.typ + "_default",
				DisplayName:      l.displayName,
				Icon:             l.icon,
				DescriptionShort: l.descShort,
				DefaultParams:    json.RawMessage(`{}`),
			},
		},
	}
}

// Resolve delegates to the bridge installed by package sections (see Task 1.8).
func (l *libStaple) Resolve(rc ResolverContext) (ResolvedItems, error) {
	return delegateResolve(l.typ, rc)
}

func init() {
	Register(&libStaple{typ: "recently_added", displayName: "Recently Added", icon: "🆕", descShort: "Latest additions to your library.", cacheTTL: 5 * time.Minute})
	Register(&libStaple{typ: "recently_released", displayName: "New Releases", icon: "🎬", descShort: "Recently released titles.", cacheTTL: 30 * time.Minute})
	Register(&libStaple{typ: "continue_watching", displayName: "Continue Watching", icon: "▶️", descShort: "Pick up where you left off.", cacheTTL: time.Minute})
	Register(&libStaple{typ: "next_up", displayName: "On Deck", icon: "📺", descShort: "Next episodes ready to watch.", cacheTTL: time.Minute})
	Register(&libStaple{typ: "watchlist", displayName: "Watchlist", icon: "🔖", descShort: "Items you've saved to watch.", cacheTTL: time.Minute})
	Register(&libStaple{typ: "favorites", displayName: "Favorites", icon: "⭐", descShort: "Your favorites.", cacheTTL: time.Minute})
	Register(&libStaple{typ: "random", displayName: "Surprise Me", icon: "🎲", descShort: "A random selection from your library.", cacheTTL: 5 * time.Minute})
}
