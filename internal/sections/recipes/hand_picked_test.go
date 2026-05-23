package recipes

import (
	"encoding/json"
	"testing"
)

func TestCollectionRecipeRegistered(t *testing.T) {
	rec, ok := Get("collection")
	if !ok {
		t.Fatal("collection not registered")
	}
	if rec.Definition().Category != CategoryHandPicked {
		t.Errorf("category = %v want hand_picked", rec.Definition().Category)
	}
}

func TestCollectionRequiresLibraryCollectionID(t *testing.T) {
	rec, _ := Get("collection")
	if err := rec.Validate(json.RawMessage(`{}`)); err == nil {
		t.Error("expected validation error when library_collection_id missing")
	}
	good := json.RawMessage(`{"library_collection_id":"42"}`)
	if err := rec.Validate(good); err != nil {
		t.Errorf("good config rejected: %v", err)
	}
}
