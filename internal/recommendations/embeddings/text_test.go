package embeddings

import (
	"strings"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestBuildEmbeddingTextSortsCrewDeterministically(t *testing.T) {
	item := &models.MediaItem{
		Title: "Example",
		Type:  "movie",
		People: []models.ItemPerson{
			{Person: models.Person{ID: 3, Name: "Charlie"}, Kind: models.PersonKindWriter, SortOrder: 0},
			{Person: models.Person{ID: 1, Name: "Alice"}, Kind: models.PersonKindWriter, SortOrder: 0},
			{Person: models.Person{ID: 2, Name: "Bob"}, Kind: models.PersonKindWriter, SortOrder: 0},
			{Person: models.Person{ID: 5, Name: "Zed"}, Kind: models.PersonKindDirector, SortOrder: 0},
			{Person: models.Person{ID: 4, Name: "Ann"}, Kind: models.PersonKindDirector, SortOrder: 0},
		},
	}

	text := BuildEmbeddingText(item)

	if !strings.Contains(text, "Directed by Ann, Zed") {
		t.Fatalf("director order was not deterministic by name: %q", text)
	}
	if !strings.Contains(text, "Written by Alice, Bob, Charlie") {
		t.Fatalf("writer order was not deterministic by name: %q", text)
	}
}

func TestBuildEmbeddingTextSortsActorsBeforeTopFive(t *testing.T) {
	item := &models.MediaItem{
		Title: "Example",
		Type:  "movie",
		People: []models.ItemPerson{
			{Person: models.Person{ID: 6, Name: "F"}, Kind: models.PersonKindActor, Character: "Six", SortOrder: 6},
			{Person: models.Person{ID: 2, Name: "B"}, Kind: models.PersonKindActor, Character: "Two", SortOrder: 2},
			{Person: models.Person{ID: 1, Name: "A"}, Kind: models.PersonKindActor, Character: "One", SortOrder: 1},
			{Person: models.Person{ID: 5, Name: "E"}, Kind: models.PersonKindActor, Character: "Five", SortOrder: 5},
			{Person: models.Person{ID: 4, Name: "D"}, Kind: models.PersonKindActor, Character: "Four", SortOrder: 4},
			{Person: models.Person{ID: 3, Name: "C"}, Kind: models.PersonKindActor, Character: "Three", SortOrder: 3},
		},
	}

	text := BuildEmbeddingText(item)

	if strings.Contains(text, "F as Six") {
		t.Fatalf("sixth actor should not be included after sorting: %q", text)
	}
	if !strings.Contains(text, "Cast: A as One, B as Two, C as Three, D as Four, E as Five") {
		t.Fatalf("actor order was not deterministic by sort order: %q", text)
	}
}
