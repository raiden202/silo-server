package recipes

import (
	"encoding/json"
	"testing"
)

func TestMoodRecipeRegistered(t *testing.T) {
	rec, ok := Get("mood_collection")
	if !ok {
		t.Fatal("mood_collection not registered")
	}
	if rec.Definition().Category != CategoryMood {
		t.Errorf("category = %v want mood", rec.Definition().Category)
	}
	if !rec.Definition().AvoidDuplicates {
		t.Error("mood_collection should set AvoidDuplicates")
	}
}

func TestMoodValidatesKnownMoods(t *testing.T) {
	rec, _ := Get("mood_collection")
	if err := rec.Validate(json.RawMessage(`{"mood":"feel_good"}`)); err != nil {
		t.Errorf("feel_good rejected: %v", err)
	}
	if err := rec.Validate(json.RawMessage(`{"mood":"made_up"}`)); err == nil {
		t.Error("expected error for unknown mood")
	}
}

func TestMoodRequiresMood(t *testing.T) {
	rec, _ := Get("mood_collection")
	if err := rec.Validate(json.RawMessage(``)); err == nil {
		t.Error("expected error for empty raw")
	}
	if err := rec.Validate(json.RawMessage(`{}`)); err == nil {
		t.Error("expected error for missing mood")
	}
	if err := rec.Validate(json.RawMessage(`{"mood":""}`)); err == nil {
		t.Error("expected error for empty mood string")
	}
}

func TestMoodPresetsCoverAllMoods(t *testing.T) {
	rec, _ := Get("mood_collection")
	presets := rec.Definition().Presets
	if len(presets) != 8 {
		t.Fatalf("expected 8 presets, got %d", len(presets))
	}
	seen := map[string]bool{}
	for _, p := range presets {
		seen[p.Key] = true
	}
	for _, want := range []string{"mood_feel_good", "mood_mind_bending", "mood_comfort", "mood_edge_of_seat", "mood_tearjerker", "mood_quiet_sunday", "mood_date_night", "mood_after_midnight"} {
		if !seen[want] {
			t.Errorf("missing preset %q", want)
		}
	}
}

func TestMoodPresetsAreStable(t *testing.T) {
	// Calling Definition() twice should return presets in the same order.
	rec, _ := Get("mood_collection")
	keys1 := []string{}
	for _, p := range rec.Definition().Presets {
		keys1 = append(keys1, p.Key)
	}
	keys2 := []string{}
	for _, p := range rec.Definition().Presets {
		keys2 = append(keys2, p.Key)
	}
	for i := range keys1 {
		if keys1[i] != keys2[i] {
			t.Fatalf("preset order is unstable across calls: %v vs %v", keys1, keys2)
		}
	}
}
