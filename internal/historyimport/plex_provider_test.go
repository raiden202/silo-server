package historyimport

import (
	"testing"
	"time"
)

func TestParsePlexGuids(t *testing.T) {
	tests := []struct {
		name                         string
		guids                        PlexGuids
		wantIMDb, wantTMDB, wantTVDB string
	}{
		{
			name:     "standard new-style guids",
			guids:    PlexGuids{{ID: "imdb://tt0322259"}, {ID: "tmdb://584"}, {ID: "tvdb://20800"}},
			wantIMDb: "tt0322259", wantTMDB: "584", wantTVDB: "20800",
		},
		{
			name:     "empty guids",
			guids:    nil,
			wantIMDb: "", wantTMDB: "", wantTVDB: "",
		},
		{
			name:     "partial guids",
			guids:    PlexGuids{{ID: "tmdb://12345"}},
			wantIMDb: "", wantTMDB: "12345", wantTVDB: "",
		},
		{
			name:     "case insensitive provider",
			guids:    PlexGuids{{ID: "IMDB://tt9999999"}, {ID: "Tmdb://111"}},
			wantIMDb: "tt9999999", wantTMDB: "111", wantTVDB: "",
		},
		{
			name:     "first value wins on duplicate providers",
			guids:    PlexGuids{{ID: "tmdb://100"}, {ID: "tmdb://200"}},
			wantIMDb: "", wantTMDB: "100", wantTVDB: "",
		},
		{
			name:     "malformed entry ignored",
			guids:    PlexGuids{{ID: "no-separator"}, {ID: "tmdb://999"}},
			wantIMDb: "", wantTMDB: "999", wantTVDB: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var imdb, tmdb, tvdb string
			ParsePlexGuids(tt.guids, &imdb, &tmdb, &tvdb)
			if imdb != tt.wantIMDb {
				t.Errorf("imdb = %q, want %q", imdb, tt.wantIMDb)
			}
			if tmdb != tt.wantTMDB {
				t.Errorf("tmdb = %q, want %q", tmdb, tt.wantTMDB)
			}
			if tvdb != tt.wantTVDB {
				t.Errorf("tvdb = %q, want %q", tvdb, tt.wantTVDB)
			}
		})
	}
}

func TestNormalizePlexItem_Movie(t *testing.T) {
	item := PlexItem{
		RatingKey:    "65196",
		Type:         "movie",
		Title:        "2 Fast 2 Furious",
		Year:         2003,
		Duration:     6461788,
		ViewCount:    1,
		LastViewedAt: 1756839905,
		Guid:         PlexGuids{{ID: "tmdb://584"}, {ID: "imdb://tt0322259"}},
	}
	record := NormalizePlexItem(item, nil)

	if record.ExternalID != "65196" {
		t.Errorf("ExternalID = %q, want %q", record.ExternalID, "65196")
	}
	if record.Kind != KindMovie {
		t.Errorf("Kind = %q, want %q", record.Kind, KindMovie)
	}
	if record.Title != "2 Fast 2 Furious" {
		t.Errorf("Title = %q", record.Title)
	}
	if record.Year != 2003 {
		t.Errorf("Year = %d", record.Year)
	}
	if record.TMDBID != "584" {
		t.Errorf("TMDBID = %q", record.TMDBID)
	}
	if record.IMDbID != "tt0322259" {
		t.Errorf("IMDbID = %q", record.IMDbID)
	}
	if !record.Played {
		t.Error("expected Played=true")
	}
	if record.PlayCount != 1 {
		t.Errorf("PlayCount = %d", record.PlayCount)
	}
	if record.DurationSeconds < 6461 || record.DurationSeconds > 6462 {
		t.Errorf("DurationSeconds = %f, want ~6461.788", record.DurationSeconds)
	}
	if record.LastPlayedAt == nil {
		t.Fatal("expected non-nil LastPlayedAt")
	}
	expected := time.Unix(1756839905, 0).UTC()
	if !record.LastPlayedAt.Equal(expected) {
		t.Errorf("LastPlayedAt = %v, want %v", record.LastPlayedAt, expected)
	}
}

func TestNormalizePlexItem_Episode(t *testing.T) {
	series := &PlexItem{
		Title: "Raising Hope",
		Year:  2010,
		Guid:  PlexGuids{{ID: "imdb://tt1615919"}, {ID: "tmdb://32815"}, {ID: "tvdb://164021"}},
	}
	item := PlexItem{
		RatingKey:            "57498",
		Type:                 "episode",
		Title:                "Cheaters",
		GrandparentTitle:     "Raising Hope",
		GrandparentRatingKey: "57479",
		ParentIndex:          1,
		Index:                18,
		Duration:             1297344,
		ViewCount:            2,
		LastViewedAt:         1775013724,
		ViewOffset:           67317,
		Guid:                 PlexGuids{{ID: "imdb://tt1792428"}, {ID: "tmdb://770538"}, {ID: "tvdb://3990621"}},
	}
	record := NormalizePlexItem(item, series)

	if record.Kind != KindEpisode {
		t.Errorf("Kind = %q", record.Kind)
	}
	if record.SeriesTitle != "Raising Hope" {
		t.Errorf("SeriesTitle = %q", record.SeriesTitle)
	}
	if record.SeriesYear != 2010 {
		t.Errorf("SeriesYear = %d", record.SeriesYear)
	}
	if record.SeasonNumber != 1 {
		t.Errorf("SeasonNumber = %d", record.SeasonNumber)
	}
	if record.EpisodeNumber != 18 {
		t.Errorf("EpisodeNumber = %d", record.EpisodeNumber)
	}
	if record.SeriesTMDBID != "32815" {
		t.Errorf("SeriesTMDBID = %q", record.SeriesTMDBID)
	}
	if record.SeriesTVDBID != "164021" {
		t.Errorf("SeriesTVDBID = %q", record.SeriesTVDBID)
	}
	if record.PositionSeconds < 67 || record.PositionSeconds > 68 {
		t.Errorf("PositionSeconds = %f, want ~67.317", record.PositionSeconds)
	}
}

func TestNormalizePlexItem_NoViewCount(t *testing.T) {
	item := PlexItem{
		RatingKey: "100",
		Type:      "movie",
		Title:     "Unwatched Movie",
		ViewCount: 0,
	}
	record := NormalizePlexItem(item, nil)
	if record.Played {
		t.Error("expected Played=false for ViewCount=0")
	}
}
