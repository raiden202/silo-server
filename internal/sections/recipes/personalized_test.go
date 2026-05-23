package recipes

import (
	"encoding/json"
	"testing"
)

func TestPersonalizedRecipesRegistered(t *testing.T) {
	for _, typ := range []string{"recommended_for_you", "because_you_watched", "similar_users_liked", "taste_match"} {
		rec, ok := Get(typ)
		if !ok {
			t.Errorf("recipe %q not registered", typ)
			continue
		}
		if rec.Definition().Category != CategoryPersonalized {
			t.Errorf("%q category = %v want personalized", typ, rec.Definition().Category)
		}
	}
}

func TestBecauseYouWatchedAcceptsAnchorParam(t *testing.T) {
	rec, _ := Get("because_you_watched")
	good := json.RawMessage(`{"anchor_item_id":"abc123"}`)
	if err := rec.Validate(good); err != nil {
		t.Fatalf("validate good: %v", err)
	}
	auto := json.RawMessage(`{"anchor_item_id":""}`)
	if err := rec.Validate(auto); err != nil {
		t.Fatalf("validate empty anchor (auto): %v", err)
	}
}
