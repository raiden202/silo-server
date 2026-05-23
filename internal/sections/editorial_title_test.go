package sections

import "testing"

func TestEditorialSpotlightDisplayTitleAppendsResolvedSubject(t *testing.T) {
	t.Parallel()

	got := editorialSpotlightDisplayTitle("Actor Spotlight", "Liam Neeson")
	want := "Actor Spotlight - Liam Neeson"
	if got != want {
		t.Fatalf("editorialSpotlightDisplayTitle = %q, want %q", got, want)
	}
}

func TestEditorialSpotlightDisplayTitleAvoidsDuplicateSubject(t *testing.T) {
	t.Parallel()

	got := editorialSpotlightDisplayTitle("Actor Spotlight - Liam Neeson", "Liam Neeson")
	want := "Actor Spotlight - Liam Neeson"
	if got != want {
		t.Fatalf("editorialSpotlightDisplayTitle = %q, want %q", got, want)
	}
}

func TestEditorialSpotlightDisplayTitleFallsBackToBaseTitle(t *testing.T) {
	t.Parallel()

	got := editorialSpotlightDisplayTitle("Actor Spotlight", "")
	want := "Actor Spotlight"
	if got != want {
		t.Fatalf("editorialSpotlightDisplayTitle = %q, want %q", got, want)
	}
}
