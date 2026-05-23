package recipes

import (
	"encoding/json"
	"fmt"
	"time"
)

// CustomFilterParams is the typed config shape for genre and custom_filter sections.
type CustomFilterParams struct {
	Filter              json.RawMessage `json:"filter,omitempty"`
	FilterType          string          `json:"filter_type,omitempty"`           // movies|series|all
	FilterLibrary       []int           `json:"filter_library,omitempty"`        // multi-library scope
	LibraryCollectionID string          `json:"library_collection_id,omitempty"` // collection sections
}

type customFilterRecipe struct {
	typ      string
	cacheTTL time.Duration
}

func (c *customFilterRecipe) Type() string                   { return c.typ }
func (c *customFilterRecipe) NewParams() any                 { return &CustomFilterParams{} }
func (c *customFilterRecipe) DefaultCacheTTL() time.Duration { return c.cacheTTL }
func (c *customFilterRecipe) Resolve(rc ResolverContext) (ResolvedItems, error) {
	return delegateResolve(c.typ, rc)
}

func (c *customFilterRecipe) Validate(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var p CustomFilterParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	// Filter, when present, must be a JSON object (not a scalar or array).
	if len(p.Filter) > 0 {
		var probe any
		if err := json.Unmarshal(p.Filter, &probe); err != nil {
			return err
		}
		if _, ok := probe.(map[string]any); !ok {
			return fmt.Errorf("recipes: filter must be a JSON object, got %T", probe)
		}
	}
	return nil
}

func (c *customFilterRecipe) Definition() RecipeDefinition {
	return RecipeDefinition{
		Type:      c.typ,
		Category:  CategoryCustom,
		AdminOnly: true,             // gated by sections.allow_profile_custom_sections at the API layer
		Hidden:    c.typ == "genre", // legacy alias — keep resolvable, hide from UI
	}
}

func init() {
	Register(&customFilterRecipe{typ: "genre", cacheTTL: 5 * time.Minute})
	Register(&customFilterRecipe{typ: "custom_filter", cacheTTL: 5 * time.Minute})
}
