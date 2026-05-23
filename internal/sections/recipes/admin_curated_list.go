package recipes

import (
	"encoding/json"
	"errors"
	"time"
)

// AdminCuratedListParams configures admin_curated_list.
type AdminCuratedListParams struct {
	ItemIDs []string `json:"item_ids"`
}

type adminCuratedListRecipe struct{}

func (adminCuratedListRecipe) Type() string                   { return "admin_curated_list" }
func (adminCuratedListRecipe) NewParams() any                 { return &AdminCuratedListParams{} }
func (adminCuratedListRecipe) DefaultCacheTTL() time.Duration { return time.Hour }
func (adminCuratedListRecipe) Resolve(rc ResolverContext) (ResolvedItems, error) {
	return delegateResolve("admin_curated_list", rc)
}
func (adminCuratedListRecipe) Validate(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("admin_curated_list: config is required")
	}
	var p AdminCuratedListParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return err
	}
	if len(p.ItemIDs) == 0 {
		return errors.New("admin_curated_list: item_ids must not be empty")
	}
	return nil
}
func (adminCuratedListRecipe) Definition() RecipeDefinition {
	return RecipeDefinition{
		Type:      "admin_curated_list",
		Category:  CategoryHandPicked,
		AdminOnly: true,
		Presets: []GalleryPreset{
			{
				Key:              "acl_blank",
				DisplayName:      "Editor's Picks",
				Icon:             "✏️",
				DescriptionShort: "Admin-curated, item-by-item ordered list.",
				DefaultParams:    json.RawMessage(`{"item_ids":[]}`),
			},
		},
	}
}

func init() {
	Register(adminCuratedListRecipe{})
}
