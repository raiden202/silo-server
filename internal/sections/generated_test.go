package sections

import (
	"testing"
)

func TestParseGeneratedHomeLibraryRecentConfig(t *testing.T) {
	libraryID, ok := ParseGeneratedHomeLibraryRecentConfig(GeneratedHomeLibraryRecentConfig(42))
	if !ok {
		t.Fatalf("expected config to parse")
	}
	if libraryID != 42 {
		t.Fatalf("library id = %d, want 42", libraryID)
	}
}

func TestShouldSyncGeneratedHomeLibraryRecentTitle(t *testing.T) {
	section := &PageSection{
		Scope:       "home",
		SectionType: SectionRecentlyAdded,
		Title:       "Recently Added in Movies",
		Config:      GeneratedHomeLibraryRecentConfig(3),
	}
	if !ShouldSyncGeneratedHomeLibraryRecentTitle(section, "Movies") {
		t.Fatalf("expected generated title to sync")
	}

	section.Title = "Staff Picks"
	if ShouldSyncGeneratedHomeLibraryRecentTitle(section, "Movies") {
		t.Fatalf("did not expect custom title to sync")
	}
}
