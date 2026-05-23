package historyimport

import (
	"testing"
	"time"
)

func TestNormalizeJellyfinItem_Episode(t *testing.T) {
	series := jellyfinItem{ID: "s1", Name: "The Series", ProductionYear: 2014, ProviderIDs: map[string]string{"tmdb": "100"}}
	item := jellyfinItem{ID: "e1", Type: "Episode", Name: "Pilot", SeriesName: "The Series", SeriesID: "s1", ProductionYear: 2015, RunTimeTicks: 3_000_000_000, UserData: struct {
		PlaybackPositionTicks int64      `json:"PlaybackPositionTicks"`
		PlayCount             int        `json:"PlayCount"`
		LastPlayedDate        *time.Time `json:"LastPlayedDate"`
		Played                bool       `json:"Played"`
	}{PlaybackPositionTicks: 90_000_000, PlayCount: 2, Played: true}, IndexNumber: 1, ParentIndexNumber: 1, ProviderIDs: map[string]string{"imdb": "tt1"}}
	record := normalizeJellyfinItem(item, series)
	if record.Kind != KindEpisode || record.SeriesTitle != "The Series" || record.SeriesYear != 2014 || record.SeriesTMDBID != "100" || record.IMDbID != "tt1" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.SeasonNumber != 1 || record.EpisodeNumber != 1 {
		t.Fatalf("unexpected episode numbers: %+v", record)
	}
	if record.PositionSeconds < 9 || record.PositionSeconds > 9.1 {
		t.Fatalf("position seconds = %f", record.PositionSeconds)
	}
}

func TestJellyfinProviderFetch_MergesDuplicates(t *testing.T) {
	first := Record{ExternalID: "1", Kind: KindMovie, Title: "Movie", Played: false, PlayCount: 1, PositionSeconds: 10, DurationSeconds: 100, UpdatedAt: time.Unix(1, 0).UTC()}
	second := Record{ExternalID: "1", Kind: KindMovie, Title: "Movie", Played: true, PlayCount: 2, PositionSeconds: 20, DurationSeconds: 120, UpdatedAt: time.Unix(2, 0).UTC()}
	merged := mergeRecords(first, second)
	if !merged.Played || merged.PlayCount != 2 || merged.PositionSeconds != 20 || merged.DurationSeconds != 120 || !merged.UpdatedAt.Equal(time.Unix(2, 0).UTC()) {
		t.Fatalf("unexpected merged record: %+v", merged)
	}
}
