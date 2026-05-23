package metadata

import (
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestIsEpisodePlaceholderTitle(t *testing.T) {
	cases := map[string]bool{
		"":           false,
		"Pilot":      false,
		"TBA":        true,
		"tbd":        true,
		"Episode 12": true,
	}

	for title, want := range cases {
		if got := IsEpisodePlaceholderTitle(title); got != want {
			t.Fatalf("title %q placeholder=%v want %v", title, got, want)
		}
	}
}

func TestEpisodeHasIncompleteMetadata(t *testing.T) {
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	complete := &models.Episode{
		Title:          "Pilot",
		Overview:       "Overview",
		StillPath:      "s3://still.jpg",
		MetadataSource: "provider",
	}
	if EpisodeHasIncompleteMetadata(complete, now) {
		t.Fatal("expected complete episode to be treated as complete")
	}

	recent := now.Add(-7 * 24 * time.Hour)
	incomplete := &models.Episode{
		Title:          "Episode 2",
		Overview:       "",
		AirDate:        &recent,
		MetadataSource: "scanner_fallback",
	}
	if !EpisodeHasIncompleteMetadata(incomplete, now) {
		t.Fatal("expected fallback placeholder episode to be treated as incomplete")
	}
}
