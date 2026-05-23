package historyimport

import (
	"encoding/json"
	"testing"
)

func TestPlexItemGuidUnmarshal_String(t *testing.T) {
	var item PlexItem
	err := json.Unmarshal([]byte(`{"ratingKey":"65196","Guid":"imdb://tt0322259"}`), &item)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(item.Guid) != 1 {
		t.Fatalf("expected 1 guid, got %d", len(item.Guid))
	}
	if item.Guid[0].ID != "imdb://tt0322259" {
		t.Fatalf("guid = %q, want %q", item.Guid[0].ID, "imdb://tt0322259")
	}
}

func TestPlexItemGuidUnmarshal_Array(t *testing.T) {
	var item PlexItem
	err := json.Unmarshal([]byte(`{"ratingKey":"65196","Guid":[{"id":"imdb://tt0322259"},{"id":"tmdb://584"}]}`), &item)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(item.Guid) != 2 {
		t.Fatalf("expected 2 guids, got %d", len(item.Guid))
	}
	if item.Guid[0].ID != "imdb://tt0322259" {
		t.Fatalf("first guid = %q, want %q", item.Guid[0].ID, "imdb://tt0322259")
	}
	if item.Guid[1].ID != "tmdb://584" {
		t.Fatalf("second guid = %q, want %q", item.Guid[1].ID, "tmdb://584")
	}
}
