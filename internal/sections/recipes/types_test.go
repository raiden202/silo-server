package recipes

import (
	"encoding/json"
	"testing"
)

func TestRecipeDefinitionMarshalsJSON(t *testing.T) {
	def := RecipeDefinition{
		Type:     "example",
		Category: CategoryDiscovery,
		Presets: []GalleryPreset{
			{Key: "p1", DisplayName: "Preset 1", Icon: "💎", DescriptionShort: "short", DefaultParams: json.RawMessage(`{}`)},
		},
		AvoidDuplicates:  true,
		SupportsRotation: false,
		AdminOnly:        false,
	}
	b, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(b) == 0 {
		t.Fatal("empty marshal")
	}
}

func TestCategoryConstantsExposed(t *testing.T) {
	if CategoryLibraryStaples == "" || CategoryPersonalized == "" || CategoryDiscovery == "" || CategoryEditorial == "" || CategorySeasonal == "" || CategoryMood == "" || CategoryHandPicked == "" || CategorySocial == "" || CategoryCustom == "" {
		t.Fatal("category constants missing")
	}
}
