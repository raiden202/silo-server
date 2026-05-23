package recipes

import (
	"encoding/json"
	"testing"
)

func TestSocialRecipesRegistered(t *testing.T) {
	for _, typ := range []string{"trending_on_server", "profile_activity_feed", "new_to_library", "most_watched"} {
		if _, ok := Get(typ); !ok {
			t.Errorf("%s not registered", typ)
		}
	}
}

func TestSocialRecipesAreSocialCategory(t *testing.T) {
	for _, typ := range []string{"trending_on_server", "profile_activity_feed", "new_to_library", "most_watched"} {
		rec, _ := Get(typ)
		if rec.Definition().Category != CategorySocial {
			t.Errorf("%s category = %v want social", typ, rec.Definition().Category)
		}
	}
}

func TestTrendingValidatesWindow(t *testing.T) {
	rec, _ := Get("trending_on_server")
	for _, good := range []string{`{}`, `{"window":""}`, `{"window":"24h"}`, `{"window":"7d"}`, `{"window":"30d"}`} {
		if err := rec.Validate(json.RawMessage(good)); err != nil {
			t.Errorf("good params %s rejected: %v", good, err)
		}
	}
	if err := rec.Validate(json.RawMessage(`{"window":"forever"}`)); err == nil {
		t.Error("expected error for invalid window")
	}
}

func TestMostWatchedValidatesWindow(t *testing.T) {
	rec, _ := Get("most_watched")
	for _, good := range []string{`{}`, `{"window":""}`, `{"window":"week"}`, `{"window":"month"}`} {
		if err := rec.Validate(json.RawMessage(good)); err != nil {
			t.Errorf("good params %s rejected: %v", good, err)
		}
	}
	if err := rec.Validate(json.RawMessage(`{"window":"year"}`)); err == nil {
		t.Error("expected error for invalid window")
	}
}

func TestProfileActivityFeedAcceptsEmptyOrPinned(t *testing.T) {
	rec, _ := Get("profile_activity_feed")
	if err := rec.Validate(json.RawMessage(`{}`)); err != nil {
		t.Errorf("empty config rejected: %v", err)
	}
	if err := rec.Validate(json.RawMessage(`{"profile_id":"abc-123"}`)); err != nil {
		t.Errorf("pinned profile rejected: %v", err)
	}
}

func TestNewToLibraryAcceptsLookback(t *testing.T) {
	rec, _ := Get("new_to_library")
	if err := rec.Validate(json.RawMessage(`{"lookback_days":30}`)); err != nil {
		t.Errorf("good params rejected: %v", err)
	}
	if err := rec.Validate(json.RawMessage(`{}`)); err != nil {
		t.Errorf("empty config rejected: %v", err)
	}
}

func TestSocialPresetsCounts(t *testing.T) {
	cases := map[string]int{"trending_on_server": 3, "profile_activity_feed": 1, "new_to_library": 1, "most_watched": 2}
	for typ, want := range cases {
		rec, _ := Get(typ)
		if got := len(rec.Definition().Presets); got != want {
			t.Errorf("%s presets = %d want %d", typ, got, want)
		}
	}
}
