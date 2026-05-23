package historyimport

import (
	"testing"
	"time"
)

func TestNormalizeEmbyItemWithoutLastPlayedDateHasNoFreshnessTimestamp(t *testing.T) {
	t.Parallel()

	item := embyItem{
		ID:           "emby-episode-1",
		Name:         "Stale partial",
		Type:         "Episode",
		SeriesName:   "The Show",
		SeriesID:     "emby-series-1",
		RunTimeTicks: 3_000_000_000,
	}
	item.UserData.PlaybackPositionTicks = 1_200_000_000

	record := normalizeEmbyItem(item, embyItem{Name: "The Show"})

	if !record.UpdatedAt.IsZero() {
		t.Fatalf("UpdatedAt = %v, want zero for resumable without LastPlayedDate", record.UpdatedAt)
	}
	if record.PositionSeconds != 120 {
		t.Fatalf("PositionSeconds = %v, want 120", record.PositionSeconds)
	}
}

func TestNormalizeEmbyItemUsesLastPlayedDateForFreshness(t *testing.T) {
	t.Parallel()

	lastPlayed := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	item := embyItem{ID: "emby-episode-1", Name: "Played", Type: "Episode"}
	item.UserData.LastPlayedDate = &lastPlayed
	item.UserData.Played = true

	record := normalizeEmbyItem(item, embyItem{})

	if !record.UpdatedAt.Equal(lastPlayed) {
		t.Fatalf("UpdatedAt = %v, want %v", record.UpdatedAt, lastPlayed)
	}
	if record.LastPlayedAt == nil || !record.LastPlayedAt.Equal(lastPlayed) {
		t.Fatalf("LastPlayedAt = %v, want %v", record.LastPlayedAt, lastPlayed)
	}
}
