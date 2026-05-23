package historyimport

import (
	"reflect"
	"testing"
)

func TestPersistedWarnings_AppendsUnmatchedReasonSummary(t *testing.T) {
	t.Parallel()

	got := persistedWarnings(ExecutionSummary{
		Warnings: []string{"upstream retry while fetching metadata"},
		UnmatchedReasonCounts: map[string]int{
			"missing season or episode number":       7,
			"ambiguous tmdb_id match for \"295693\"": 1,
		},
	})

	want := []string{
		"upstream retry while fetching metadata",
		"unmatched items (7): missing season or episode number",
		"unmatched items (1): ambiguous tmdb_id match for \"295693\"",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("persistedWarnings() = %#v, want %#v", got, want)
	}
}
