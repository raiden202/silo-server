package jellycompat

import (
	"testing"

	"github.com/Silo-Server/silo-server/internal/config"
)

func TestItemDetailSortNamePrefersSortTitle(t *testing.T) {
	m := newMapper(NewResourceIDCodec(), &config.Config{})
	dto := m.itemFromDetail(upstreamItemDetail{
		ContentID:     "movie-1",
		Type:          "movie",
		Title:         "The Matrix",
		SortTitle:     "Matrix, The",
		OriginalTitle: "Matrix Original",
	}, false, nil)
	if dto.SortName != "Matrix, The" {
		t.Fatalf("SortName = %q, want %q", dto.SortName, "Matrix, The")
	}
	if dto.ForcedSortName != "Matrix, The" {
		t.Fatalf("ForcedSortName = %q, want %q", dto.ForcedSortName, "Matrix, The")
	}
}

func TestItemListSortNamePrefersSortTitle(t *testing.T) {
	m := newMapper(NewResourceIDCodec(), &config.Config{})
	dto := m.itemFromList(upstreamListItem{
		ContentID: "movie-1",
		Type:      "movie",
		Title:     "The Matrix",
		SortTitle: "Matrix, The",
	}, false, nil, map[string]bool{"sortname": true})
	if dto.SortName != "Matrix, The" {
		t.Fatalf("SortName = %q, want %q", dto.SortName, "Matrix, The")
	}
}
