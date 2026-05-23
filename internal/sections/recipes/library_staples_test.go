package recipes

import (
	"encoding/json"
	"testing"
)

func TestLibraryStaplesAreRegistered(t *testing.T) {
	wanted := []string{"recently_added", "recently_released", "continue_watching", "next_up", "watchlist", "favorites", "random"}
	for _, typ := range wanted {
		rec, ok := Get(typ)
		if !ok {
			t.Errorf("recipe %q not registered", typ)
			continue
		}
		if rec.Type() != typ {
			t.Errorf("recipe %q.Type() = %q", typ, rec.Type())
		}
		if rec.Definition().Category != CategoryLibraryStaples {
			t.Errorf("recipe %q category = %v want %v", typ, rec.Definition().Category, CategoryLibraryStaples)
		}
	}
}

func TestLibraryStaplesAcceptEmptyParams(t *testing.T) {
	for _, typ := range []string{"recently_added", "continue_watching", "random"} {
		rec, _ := Get(typ)
		if err := rec.Validate(json.RawMessage(`{}`)); err != nil {
			t.Errorf("%s.Validate({}) = %v", typ, err)
		}
	}
}
