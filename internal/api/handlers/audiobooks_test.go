package handlers

import (
	"context"
	"net/url"
	"testing"

	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/models"
)

func TestPresignAudiobookPosterDoesNotExposeRawKeysWithoutPresigner(t *testing.T) {
	h := &AudiobookHandler{}
	got := h.presignAudiobookPoster(context.Background(), "metadata/audiobooks/book-1/poster/original.webp")
	if got != "" {
		t.Fatalf("PosterURL = %q, want empty without presigner", got)
	}
}

func TestAudiobookCatalogRequestScopesToAudiobooks(t *testing.T) {
	req, err := audiobookCatalogRequest(mapValues(
		"genre", "Fantasy",
		"offset", "10",
	))
	if err != nil {
		t.Fatalf("audiobookCatalogRequest returned error: %v", err)
	}
	if req.Source != catalog.CatalogSourceQuery {
		t.Fatalf("Source = %q, want query", req.Source)
	}
	if req.Query.MediaScope != "audiobook" {
		t.Fatalf("MediaScope = %q, want audiobook", req.Query.MediaScope)
	}
	if req.Limit != 20 || req.Offset != 10 {
		t.Fatalf("limit/offset = %d/%d, want generic catalog defaults 20/10", req.Limit, req.Offset)
	}
	if req.Query.Sort.Field != "added_at" || req.Query.Sort.Order != "desc" {
		t.Fatalf("sort = %+v, want generic catalog defaults added_at desc", req.Query.Sort)
	}
	if len(req.Query.Groups) != 1 || len(req.Query.Groups[0].Rules) != 1 {
		t.Fatalf("groups = %+v, want one genre rule", req.Query.Groups)
	}
	rule := req.Query.Groups[0].Rules[0]
	if rule.Field != "genre" || rule.Op != "contains" || rule.Value != "Fantasy" {
		t.Fatalf("genre rule = %+v", rule)
	}
}

func TestAudiobookCatalogRequestPreservesExplicitCatalogOptions(t *testing.T) {
	req, err := audiobookCatalogRequest(mapValues(
		"limit", "75",
		"sort", "author",
		"order", "desc",
		"q", "martian",
	))
	if err != nil {
		t.Fatalf("audiobookCatalogRequest returned error: %v", err)
	}
	if req.Limit != 75 {
		t.Fatalf("Limit = %d, want 75", req.Limit)
	}
	if req.SearchQuery != "martian" {
		t.Fatalf("SearchQuery = %q, want martian", req.SearchQuery)
	}
	if req.Query.Sort.Field != "author" || req.Query.Sort.Order != "desc" {
		t.Fatalf("sort = %+v, want author desc", req.Query.Sort)
	}
	if req.Query.MediaScope != "audiobook" {
		t.Fatalf("MediaScope = %q, want audiobook", req.Query.MediaScope)
	}
}

func TestAudiobookDurationForProgress(t *testing.T) {
	files := []*models.MediaFile{
		{ID: 10, Duration: 120},
		{ID: 11, Duration: 180},
	}

	if got := audiobookDurationForProgress(files, 10); got != 120 {
		t.Fatalf("selected file duration = %v, want 120", got)
	}
	if got := audiobookDurationForProgress(files, 0); got != 300 {
		t.Fatalf("total duration = %v, want 300", got)
	}
	if got := audiobookDurationForProgress([]*models.MediaFile{{ID: 1}}, 1); got != 0 {
		t.Fatalf("unknown duration = %v, want 0", got)
	}
}

func mapValues(kv ...string) url.Values {
	out := make(url.Values)
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = []string{kv[i+1]}
	}
	return out
}
