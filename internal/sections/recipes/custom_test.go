package recipes

import (
	"encoding/json"
	"testing"
)

func TestCustomRecipesRegistered(t *testing.T) {
	for _, typ := range []string{"genre", "custom_filter"} {
		rec, ok := Get(typ)
		if !ok {
			t.Errorf("recipe %q not registered", typ)
			continue
		}
		if rec.Definition().Category != CategoryCustom {
			t.Errorf("%q category = %v want custom", typ, rec.Definition().Category)
		}
	}
}

func TestCustomFilterValidatesFilterShape(t *testing.T) {
	rec, _ := Get("custom_filter")
	good := json.RawMessage(`{"filter":{"match":"all","groups":[]}}`)
	if err := rec.Validate(good); err != nil {
		t.Errorf("good config rejected: %v", err)
	}
	bad := json.RawMessage(`{"filter":42}`)
	if err := rec.Validate(bad); err == nil {
		t.Errorf("bad config accepted")
	}
}
