package metadata

import "testing"

func TestMergeMetadataGenresFillEmptyKeepsFirstProviderGenres(t *testing.T) {
	target := &MetadataResult{
		Genres: []string{"Science Fiction", "Fantasy", "Drama"},
	}
	source := &MetadataResult{
		Genres: []string{"Crime", "Sci-Fi & Fantasy", "Action & Adventure"},
	}

	MergeMetadata(source, target, nil, MergeFillEmpty)

	if len(target.Genres) != 3 {
		t.Fatalf("expected 3 genres, got %d: %#v", len(target.Genres), target.Genres)
	}

	want := []string{"Science Fiction", "Fantasy", "Drama"}
	for i := range want {
		if target.Genres[i] != want[i] {
			t.Fatalf("genre %d = %q, want %q", i, target.Genres[i], want[i])
		}
	}
}

func TestMergeMetadataGenresReplaceUnlockedReplacesGenres(t *testing.T) {
	target := &MetadataResult{
		Genres: []string{"Science Fiction", "Fantasy", "Drama"},
	}
	source := &MetadataResult{
		Genres: []string{"Crime", "Documentary"},
	}

	MergeMetadata(source, target, nil, MergeReplaceUnlocked)

	want := []string{"Crime", "Documentary"}
	if len(target.Genres) != len(want) {
		t.Fatalf("expected %d genres, got %d: %#v", len(want), len(target.Genres), target.Genres)
	}
	for i := range want {
		if target.Genres[i] != want[i] {
			t.Fatalf("genre %d = %q, want %q", i, target.Genres[i], want[i])
		}
	}
}
