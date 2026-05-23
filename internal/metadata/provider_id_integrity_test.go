package metadata

import (
	"testing"
	"time"
)

func TestBetterCanonicalCandidatePrefersActiveFilesThenLibrariesThenAgeThenID(t *testing.T) {
	now := time.Now()
	base := canonicalCandidate{ContentID: "b", CreatedAt: now, ActiveFiles: 0, LibraryCount: 2}

	if !betterCanonicalCandidate(canonicalCandidate{ContentID: "a", CreatedAt: now, ActiveFiles: 1, LibraryCount: 0}, base) {
		t.Fatal("expected active files to win over library count")
	}
	if !betterCanonicalCandidate(canonicalCandidate{ContentID: "a", CreatedAt: now, ActiveFiles: 0, LibraryCount: 3}, base) {
		t.Fatal("expected larger library count to win")
	}
	if !betterCanonicalCandidate(canonicalCandidate{ContentID: "a", CreatedAt: now.Add(-time.Hour), ActiveFiles: 0, LibraryCount: 2}, base) {
		t.Fatal("expected older created_at to win")
	}
	if !betterCanonicalCandidate(canonicalCandidate{ContentID: "a", CreatedAt: now, ActiveFiles: 0, LibraryCount: 2}, base) {
		t.Fatal("expected lexicographically smaller content_id to win")
	}
}

func TestProviderEntriesFromDriftUsesDurableProvidersOnly(t *testing.T) {
	entries := providerEntriesFromDrift(providerIDDriftRow{
		ContentID: "item-1",
		ItemType:  "movie",
		TMDBID:    " 603 ",
		TVDBID:    "",
		IMDbID:    "tt0083658",
	})
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0] != (providerIDEntry{Provider: "tmdb", ProviderID: "603"}) {
		t.Fatalf("entries[0] = %#v", entries[0])
	}
	if entries[1] != (providerIDEntry{Provider: "imdb", ProviderID: "tt0083658"}) {
		t.Fatalf("entries[1] = %#v", entries[1])
	}
}

func TestBothCandidatesHaveContentDetectsRealUserData(t *testing.T) {
	withFiles := canonicalCandidate{ContentID: "a", ActiveFiles: 2}
	withLibraries := canonicalCandidate{ContentID: "b", LibraryCount: 1}
	skeleton := canonicalCandidate{ContentID: "c"}

	if !bothCandidatesHaveContent(withFiles, withLibraries) {
		t.Fatal("expected both-content when one has files and the other has library memberships")
	}
	if !bothCandidatesHaveContent(withFiles, withFiles) {
		t.Fatal("expected both-content when both sides have files")
	}
	if bothCandidatesHaveContent(withFiles, skeleton) {
		t.Fatal("did not expect both-content when one side is a skeleton")
	}
	if bothCandidatesHaveContent(skeleton, skeleton) {
		t.Fatal("did not expect both-content when both sides are skeletons")
	}
}
