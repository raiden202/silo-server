package catalog

import (
	"strings"
	"testing"
)

// TestEpisodeRepo_ListBySeriesGroupedBySeason_BuildsSingleQuery asserts the
// SQL shape produced by buildListBySeriesGroupedBySeasonQuery: a single
// top-level SELECT against episodes filtered by series_id, ordered by
// season+episode, with the seriesID bound as $1. This is the query that
// replaces N per-season ListBySeason calls in the jellycompat ListSeasons
// handler (audit 2026-05-01 §2.2).
func TestEpisodeRepo_ListBySeriesGroupedBySeason_BuildsSingleQuery(t *testing.T) {
	repo := &EpisodeRepository{}
	sql, args := repo.buildListBySeriesGroupedBySeasonQuery("series-abc")

	if !strings.Contains(sql, "WHERE series_id = $1") {
		t.Fatalf("expected query to filter by series_id = $1; got:\n%s", sql)
	}
	if !strings.Contains(sql, "ORDER BY season_number ASC, episode_number ASC") {
		t.Fatalf("expected query to ORDER BY season_number ASC, episode_number ASC; got:\n%s", sql)
	}
	// Top-level SELECT count: episodeAvailabilityPredicate contains an
	// EXISTS (SELECT 1 FROM ...) subquery, so checking "exactly one SELECT"
	// is too strict. Instead, assert there is exactly one SELECT before the
	// FROM episodes clause.
	upper := strings.ToUpper(sql)
	fromIdx := strings.Index(upper, "FROM EPISODES")
	if fromIdx < 0 {
		t.Fatalf("expected query to FROM episodes; got:\n%s", sql)
	}
	if got := strings.Count(upper[:fromIdx], "SELECT"); got != 1 {
		t.Fatalf("expected exactly one top-level SELECT before FROM episodes; got %d in:\n%s", got, sql)
	}
	if len(args) != 1 || args[0] != "series-abc" {
		t.Fatalf("expected args == [\"series-abc\"]; got %v", args)
	}
}

// TestEpisodeRepo_BulkUpsert_TouchesSeriesLastAirDate asserts the bulk
// maintenance SQL maintains media_items.last_air_date_at across multiple
// series in one round-trip (audit 2026-05-01 §2.1 hot path #1).
func TestEpisodeRepo_BulkUpsert_TouchesSeriesLastAirDate(t *testing.T) {
	repo := &EpisodeRepository{}
	sqlText := repo.buildBatchUpdateLastAirDateSQL()
	if !strings.Contains(sqlText, "MAX(e.air_date)") {
		t.Fatalf("expected MAX(e.air_date) in batch update; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "unnest($1::text[])") {
		t.Fatalf("expected unnest($1::text[]) input enumeration; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "IS DISTINCT FROM") {
		t.Fatalf("expected guard against unchanged-value writes; got %s", sqlText)
	}
}

// TestEpisodeRepo_BulkUpsert_ResetsToNullForSeriesWithNoQualifyingEpisodes
// pins the structural guarantee that the bulk maintenance SQL produces
// exactly one subquery row per input series_id — even when no episodes
// match — by driving from UNNEST(input) with a LEFT JOIN against episodes.
//
// Without this, a series whose entire episode set has air_date NULL or in
// the future would be dropped from the GROUP BY result and the UPDATE
// would skip it, leaving a stale last_air_date_at from an earlier sync.
// The single-item form (updateSeriesLastAirDateSQL) is naturally immune
// because its bare-MAX subquery always emits one row.
//
// Regression guard for the post-perf-overhaul code review (Codex P1).
func TestEpisodeRepo_BulkUpsert_ResetsToNullForSeriesWithNoQualifyingEpisodes(t *testing.T) {
	repo := &EpisodeRepository{}
	sqlText := repo.buildBatchUpdateLastAirDateSQL()
	// UNNEST drives the subquery so every input series_id is enumerated.
	if !strings.Contains(sqlText, "FROM unnest($1::text[])") {
		t.Fatalf("expected FROM unnest($1::text[]) as the subquery driver; got %s", sqlText)
	}
	// LEFT JOIN against episodes preserves the series row when no episodes
	// match — MAX returns NULL, and IS DISTINCT FROM correctly compares
	// NULL vs current value to perform the reset.
	if !strings.Contains(sqlText, "LEFT JOIN episodes") {
		t.Fatalf("expected LEFT JOIN against episodes (so no-match series get NULL); got %s", sqlText)
	}
	// Reject the prior broken form that filtered episodes inline and
	// GROUPed by e.series_id, which silently dropped no-match series.
	if strings.Contains(sqlText, "WHERE e.series_id = ANY($1::text[])") {
		t.Fatalf("found prior inner-filter form that drops no-match series; got %s", sqlText)
	}
}

// TestEpisodeRepo_Upsert_TouchesSeriesLastAirDate asserts the single-series
// maintenance SQL filters by content_id, restricts updates to series rows,
// and avoids writes when the value is unchanged.
func TestEpisodeRepo_Upsert_TouchesSeriesLastAirDate(t *testing.T) {
	repo := &EpisodeRepository{}
	sqlText := repo.buildUpdateSeriesLastAirDateSQL()
	if !strings.Contains(sqlText, "MAX(e.air_date)") {
		t.Fatalf("expected MAX(e.air_date) in single-series update; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "e.series_id = $1") {
		t.Fatalf("expected single-series filter on series_id = $1; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "mi.content_id = $1") {
		t.Fatalf("expected media_items filter on content_id = $1; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "mi.type = 'series'") {
		t.Fatalf("expected restriction to series rows; got %s", sqlText)
	}
	if !strings.Contains(sqlText, "IS DISTINCT FROM") {
		t.Fatalf("expected guard against unchanged-value writes; got %s", sqlText)
	}
}
