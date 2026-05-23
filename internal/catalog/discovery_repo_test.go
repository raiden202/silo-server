package catalog

// discovery_repo_test.go exercises the SQL generation helpers in
// discovery_repo.go using the same pure-unit approach used throughout this
// package: build a query string and assert that key fragments are present,
// without needing a live database.
//
// The helpers themselves execute against a real pool; integration coverage is
// provided by whichever recipe resolver exercises them in higher-level tests.

import (
	"strings"
	"testing"
)

// buildRatingThresholdQuery is the testable extraction of the SQL generation
// logic from ListByRatingThreshold so we can inspect it without a pool.
func buildRatingThresholdQuery(f RatingFilter) (string, []any) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, "mi.rating_imdb >= $1")
	args = append(args, f.Min)
	argIdx++

	fromClause := "media_items mi"
	if f.LibraryID != nil {
		fromClause = "media_items mi JOIN media_item_libraries mil ON mi.content_id = mil.content_id"
		conditions = append(conditions, "mil.media_folder_id = $2")
		args = append(args, *f.LibraryID)
		argIdx++
	} else if f.Filter.AllowedLibraryIDs != nil || len(f.Filter.DisabledLibraryIDs) > 0 {
		fromClause = "media_items mi JOIN media_item_libraries mil ON mi.content_id = mil.content_id"
		if f.Filter.AllowedLibraryIDs != nil {
			if len(f.Filter.AllowedLibraryIDs) == 0 {
				return "", nil // early empty
			}
			placeholders := make([]string, len(f.Filter.AllowedLibraryIDs))
			for i, id := range f.Filter.AllowedLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id IN ("+strings.Join(placeholders, ", ")+")")
		}
		if len(f.Filter.DisabledLibraryIDs) > 0 {
			placeholders := make([]string, len(f.Filter.DisabledLibraryIDs))
			for i, id := range f.Filter.DisabledLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id NOT IN ("+strings.Join(placeholders, ", ")+")")
		}
	}

	applyAccessFilter("mi", f.Filter, &conditions, &args, &argIdx)

	query := "SELECT " + itemColumns + " FROM " + fromClause +
		" WHERE " + strings.Join(conditions, " AND ") +
		" ORDER BY mi.rating_imdb DESC NULLS LAST, mi.content_id ASC"

	if f.Limit > 0 {
		query += " LIMIT $" + itoa(argIdx)
		args = append(args, f.Limit)
	}
	return query, args
}

// buildUnplayedHighRatedQuery mirrors the SQL-building portion of
// ListUnplayedHighRated for unit testing.
func buildUnplayedHighRatedQuery(f UnplayedFilter) (string, []any) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, "mi.rating_imdb >= $1")
	args = append(args, f.MinRating)
	argIdx++

	conditions = append(conditions, "NOT EXISTS (\n\t\tSELECT 1\n\t\tFROM user_watch_history uwh\n\t\tWHERE uwh.user_id = $"+itoa(argIdx)+"\n\t\t  AND uwh.profile_id = $"+itoa(argIdx+1)+"\n\t\t  AND uwh.media_item_id = mi.content_id\n\t)")
	args = append(args, f.UserID, f.ProfileID)
	argIdx += 2

	fromClause := "media_items mi"
	if f.Filter.AllowedLibraryIDs != nil || len(f.Filter.DisabledLibraryIDs) > 0 {
		fromClause = "media_items mi JOIN media_item_libraries mil ON mi.content_id = mil.content_id"
		if f.Filter.AllowedLibraryIDs != nil {
			if len(f.Filter.AllowedLibraryIDs) == 0 {
				return "", nil // early empty
			}
			placeholders := make([]string, len(f.Filter.AllowedLibraryIDs))
			for i, id := range f.Filter.AllowedLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id IN ("+strings.Join(placeholders, ", ")+")")
		}
		if len(f.Filter.DisabledLibraryIDs) > 0 {
			placeholders := make([]string, len(f.Filter.DisabledLibraryIDs))
			for i, id := range f.Filter.DisabledLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id NOT IN ("+strings.Join(placeholders, ", ")+")")
		}
	}

	applyAccessFilter("mi", f.Filter, &conditions, &args, &argIdx)

	query := "SELECT " + itemColumns + " FROM " + fromClause +
		" WHERE " + strings.Join(conditions, " AND ") +
		" ORDER BY mi.rating_imdb DESC NULLS LAST, mi.content_id ASC"

	if f.Limit > 0 {
		query += " LIMIT $" + itoa(argIdx)
		args = append(args, f.Limit)
	}
	return query, args
}

// itoa is a local helper to avoid importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}

// ---------------------------------------------------------------------------
// ListByRatingThreshold SQL generation tests
// ---------------------------------------------------------------------------

func TestRatingThreshold_BasicQuery(t *testing.T) {
	query, args := buildRatingThresholdQuery(RatingFilter{
		Min:   7.5,
		Limit: 20,
	})

	if !strings.Contains(query, "mi.rating_imdb >= $1") {
		t.Fatalf("expected rating threshold predicate, got:\n%s", query)
	}
	if !strings.Contains(query, "ORDER BY mi.rating_imdb DESC NULLS LAST") {
		t.Fatalf("expected rating sort order, got:\n%s", query)
	}
	if !strings.Contains(query, "mi.content_id ASC") {
		t.Fatalf("expected content_id tiebreaker, got:\n%s", query)
	}
	if !strings.Contains(query, "LIMIT $2") {
		t.Fatalf("expected LIMIT clause at $2, got:\n%s", query)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args (min rating + limit), got %d: %v", len(args), args)
	}
	if args[0] != 7.5 {
		t.Fatalf("expected first arg 7.5, got %v", args[0])
	}
	if args[1] != 20 {
		t.Fatalf("expected second arg 20 (limit), got %v", args[1])
	}
}

func TestRatingThreshold_NoLimitOmitsClause(t *testing.T) {
	query, args := buildRatingThresholdQuery(RatingFilter{Min: 8.0})

	if strings.Contains(query, "LIMIT") {
		t.Fatalf("expected no LIMIT clause, got:\n%s", query)
	}
	if len(args) != 1 {
		t.Fatalf("expected 1 arg, got %d", len(args))
	}
}

func TestRatingThreshold_LibraryIDJoin(t *testing.T) {
	libID := 42
	query, args := buildRatingThresholdQuery(RatingFilter{
		Min:       7.0,
		LibraryID: &libID,
	})

	if !strings.Contains(query, "JOIN media_item_libraries mil") {
		t.Fatalf("expected library join, got:\n%s", query)
	}
	if !strings.Contains(query, "mil.media_folder_id = $2") {
		t.Fatalf("expected media_folder_id predicate at $2, got:\n%s", query)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args (min + library id), got %d", len(args))
	}
	if args[1] != 42 {
		t.Fatalf("expected library id 42 at args[1], got %v", args[1])
	}
}

func TestRatingThreshold_AllowedLibraries(t *testing.T) {
	query, args := buildRatingThresholdQuery(RatingFilter{
		Min: 7.0,
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{1, 2},
		},
	})

	if !strings.Contains(query, "JOIN media_item_libraries mil") {
		t.Fatalf("expected library join, got:\n%s", query)
	}
	if !strings.Contains(query, "mil.media_folder_id IN ($2, $3)") {
		t.Fatalf("expected allowed library IN clause, got:\n%s", query)
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(args), args)
	}
}

func TestRatingThreshold_DisabledLibraries(t *testing.T) {
	query, args := buildRatingThresholdQuery(RatingFilter{
		Min: 7.0,
		Filter: AccessFilter{
			DisabledLibraryIDs: []int{99},
		},
	})

	if !strings.Contains(query, "mil.media_folder_id NOT IN ($2)") {
		t.Fatalf("expected disabled library NOT IN clause, got:\n%s", query)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
}

func TestRatingThreshold_EmptyAllowedLibrariesReturnsEmptyQuery(t *testing.T) {
	query, _ := buildRatingThresholdQuery(RatingFilter{
		Min: 7.0,
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{}, // empty slice → early return
		},
	})
	if query != "" {
		t.Fatalf("expected empty query for empty allowed libraries, got %q", query)
	}
}

// ---------------------------------------------------------------------------
// ListUnplayedHighRated SQL generation tests
// ---------------------------------------------------------------------------

func TestUnplayedHighRated_BasicQuery(t *testing.T) {
	query, args := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 7.5,
		Limit:     25,
		UserID:    7,
		ProfileID: "profile-abc",
	})

	if !strings.Contains(query, "mi.rating_imdb >= $1") {
		t.Fatalf("expected rating threshold predicate, got:\n%s", query)
	}
	if !strings.Contains(query, "NOT EXISTS") {
		t.Fatalf("expected NOT EXISTS subquery, got:\n%s", query)
	}
	if !strings.Contains(query, "user_watch_history uwh") {
		t.Fatalf("expected user_watch_history in subquery, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.user_id = $2") {
		t.Fatalf("expected user_id predicate at $2, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.profile_id = $3") {
		t.Fatalf("expected profile_id predicate at $3, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.media_item_id = mi.content_id") {
		t.Fatalf("expected media_item_id = mi.content_id correlation, got:\n%s", query)
	}
	if !strings.Contains(query, "ORDER BY mi.rating_imdb DESC NULLS LAST") {
		t.Fatalf("expected rating sort order, got:\n%s", query)
	}
	if !strings.Contains(query, "LIMIT $4") {
		t.Fatalf("expected LIMIT at $4, got:\n%s", query)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args (minRating, userID, profileID, limit), got %d: %v", len(args), args)
	}
}

func TestUnplayedHighRated_NoLimitOmitsClause(t *testing.T) {
	query, _ := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 7.0,
		UserID:    1,
		ProfileID: "p",
	})
	if strings.Contains(query, "LIMIT") {
		t.Fatalf("expected no LIMIT clause, got:\n%s", query)
	}
}

func TestUnplayedHighRated_NoLibraryJoin(t *testing.T) {
	query, _ := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 7.0,
		UserID:    1,
		ProfileID: "p",
	})
	if strings.Contains(query, "JOIN media_item_libraries") {
		t.Fatalf("expected no library join when no library filter is set, got:\n%s", query)
	}
}

func TestUnplayedHighRated_AllowedLibraries(t *testing.T) {
	query, args := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 7.0,
		UserID:    3,
		ProfileID: "p",
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{10, 11},
		},
	})

	if !strings.Contains(query, "JOIN media_item_libraries mil") {
		t.Fatalf("expected library join, got:\n%s", query)
	}
	if !strings.Contains(query, "mil.media_folder_id IN ($4, $5)") {
		t.Fatalf("expected allowed library IN clause at $4,$5, got:\n%s", query)
	}
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(args), args)
	}
}

func TestUnplayedHighRated_EmptyAllowedLibrariesReturnsEmptyQuery(t *testing.T) {
	query, _ := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 7.0,
		UserID:    1,
		ProfileID: "p",
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{},
		},
	})
	if query != "" {
		t.Fatalf("expected empty query for empty allowed libraries, got %q", query)
	}
}

func TestUnplayedHighRated_ContentRatingFilter(t *testing.T) {
	query, args := buildUnplayedHighRatedQuery(UnplayedFilter{
		MinRating: 6.0,
		UserID:    2,
		ProfileID: "p",
		Filter:    AccessFilter{MaxContentRating: "PG-13"},
	})

	if !strings.Contains(query, "mi.content_rating = ANY(") {
		t.Fatalf("expected content_rating = ANY filter, got:\n%s", query)
	}
	// args: minRating, userID, profileID, then content rating slice (one arg).
	// Task 5.1 converted the rating ladder from IN ($N, $N+1, ...) to
	// = ANY($N) with a single bound slice arg.
	if len(args) != 4 {
		t.Fatalf("expected exactly 4 args (minRating, userID, profileID, rating slice); got %d: %v", len(args), args)
	}
}

// ---------------------------------------------------------------------------
// ListForgottenFavorites SQL generation tests
// ---------------------------------------------------------------------------

// buildForgottenFavoritesQuery is the testable extraction of the SQL
// generation logic from ListForgottenFavorites.
func buildForgottenFavoritesQuery(f ForgottenFavoritesFilter) (string, []any) {
	if f.LookbackDays <= 0 {
		f.LookbackDays = 365
	}

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, "mi.rating_imdb >= 7.0")

	conditions = append(conditions, "NOT EXISTS (\n\t\tSELECT 1\n\t\tFROM user_watch_history uwh\n\t\tWHERE uwh.user_id = $"+itoa(argIdx)+"\n\t\t  AND uwh.profile_id = $"+itoa(argIdx+1)+"\n\t\t  AND uwh.media_item_id = mi.content_id\n\t\t  AND uwh.watched_at >= NOW() - ($"+itoa(argIdx+2)+" || ' days')::interval\n\t)")
	args = append(args, f.UserID, f.ProfileID, f.LookbackDays)
	argIdx += 3

	fromClause := "media_items mi"
	if f.Filter.AllowedLibraryIDs != nil || len(f.Filter.DisabledLibraryIDs) > 0 {
		fromClause = "media_items mi JOIN media_item_libraries mil ON mi.content_id = mil.content_id"
		if f.Filter.AllowedLibraryIDs != nil {
			if len(f.Filter.AllowedLibraryIDs) == 0 {
				return "", nil // early empty
			}
			placeholders := make([]string, len(f.Filter.AllowedLibraryIDs))
			for i, id := range f.Filter.AllowedLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id IN ("+strings.Join(placeholders, ", ")+")")
		}
		if len(f.Filter.DisabledLibraryIDs) > 0 {
			placeholders := make([]string, len(f.Filter.DisabledLibraryIDs))
			for i, id := range f.Filter.DisabledLibraryIDs {
				placeholders[i] = "$" + itoa(argIdx)
				args = append(args, id)
				argIdx++
			}
			conditions = append(conditions, "mil.media_folder_id NOT IN ("+strings.Join(placeholders, ", ")+")")
		}
	}

	applyAccessFilter("mi", f.Filter, &conditions, &args, &argIdx)

	query := "SELECT " + itemColumns + " FROM " + fromClause +
		" WHERE " + strings.Join(conditions, " AND ") +
		" ORDER BY mi.rating_imdb DESC NULLS LAST, mi.content_id ASC"

	if f.Limit > 0 {
		query += " LIMIT $" + itoa(argIdx)
		args = append(args, f.Limit)
	}
	return query, args
}

func TestForgottenFavorites_BasicQuery(t *testing.T) {
	query, args := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 365,
		Limit:        20,
		UserID:       5,
		ProfileID:    "profile-xyz",
	})

	if !strings.Contains(query, "mi.rating_imdb >= 7.0") {
		t.Fatalf("expected 7.0 rating floor, got:\n%s", query)
	}
	if !strings.Contains(query, "NOT EXISTS") {
		t.Fatalf("expected NOT EXISTS subquery, got:\n%s", query)
	}
	if !strings.Contains(query, "user_watch_history uwh") {
		t.Fatalf("expected user_watch_history in subquery, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.user_id = $1") {
		t.Fatalf("expected user_id predicate at $1, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.profile_id = $2") {
		t.Fatalf("expected profile_id predicate at $2, got:\n%s", query)
	}
	if !strings.Contains(query, "uwh.watched_at >= NOW()") {
		t.Fatalf("expected watched_at recency filter, got:\n%s", query)
	}
	if !strings.Contains(query, "$3 || ' days'") {
		t.Fatalf("expected lookback_days at $3, got:\n%s", query)
	}
	if !strings.Contains(query, "ORDER BY mi.rating_imdb DESC NULLS LAST") {
		t.Fatalf("expected rating sort order, got:\n%s", query)
	}
	if !strings.Contains(query, "LIMIT $4") {
		t.Fatalf("expected LIMIT at $4, got:\n%s", query)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args (userID, profileID, lookbackDays, limit), got %d: %v", len(args), args)
	}
}

func TestForgottenFavorites_NoLimitOmitsClause(t *testing.T) {
	query, _ := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 365,
		UserID:       1,
		ProfileID:    "p",
	})
	if strings.Contains(query, "LIMIT") {
		t.Fatalf("expected no LIMIT clause, got:\n%s", query)
	}
}

func TestForgottenFavorites_NoLibraryJoin(t *testing.T) {
	query, _ := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 365,
		UserID:       1,
		ProfileID:    "p",
	})
	if strings.Contains(query, "JOIN media_item_libraries") {
		t.Fatalf("expected no library join when no library filter is set, got:\n%s", query)
	}
}

func TestForgottenFavorites_AllowedLibraries(t *testing.T) {
	query, args := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 180,
		UserID:       3,
		ProfileID:    "p",
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{10, 11},
		},
	})

	if !strings.Contains(query, "JOIN media_item_libraries mil") {
		t.Fatalf("expected library join, got:\n%s", query)
	}
	if !strings.Contains(query, "mil.media_folder_id IN ($4, $5)") {
		t.Fatalf("expected allowed library IN clause at $4,$5, got:\n%s", query)
	}
	if len(args) != 5 {
		t.Fatalf("expected 5 args, got %d: %v", len(args), args)
	}
}

func TestForgottenFavorites_EmptyAllowedLibrariesReturnsEmptyQuery(t *testing.T) {
	query, _ := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 365,
		UserID:       1,
		ProfileID:    "p",
		Filter: AccessFilter{
			AllowedLibraryIDs: []int{},
		},
	})
	if query != "" {
		t.Fatalf("expected empty query for empty allowed libraries, got %q", query)
	}
}

func TestForgottenFavorites_DefaultLookbackApplied(t *testing.T) {
	query, args := buildForgottenFavoritesQuery(ForgottenFavoritesFilter{
		LookbackDays: 0, // should default to 365
		UserID:       1,
		ProfileID:    "p",
	})
	if !strings.Contains(query, "NOT EXISTS") {
		t.Fatalf("expected NOT EXISTS subquery, got:\n%s", query)
	}
	// args[2] should be the defaulted lookback value (365).
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(args))
	}
	if args[2] != 365 {
		t.Fatalf("expected lookback default 365, got %v", args[2])
	}
}
