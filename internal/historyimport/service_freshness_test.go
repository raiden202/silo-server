package historyimport

import (
	"testing"
	"time"
)

func TestShouldWriteImportedProgressAllowsMissingLocalProgressWithoutSourceTimestamp(t *testing.T) {
	t.Parallel()

	if !shouldWriteImportedProgress(Record{}, nil) {
		t.Fatal("expected missing local progress to be writable")
	}
}

func TestShouldWriteImportedProgressRejectsTimestamplessRecordOverLocalProgress(t *testing.T) {
	t.Parallel()

	local := &localProgressRow{UpdatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}

	if shouldWriteImportedProgress(Record{}, local) {
		t.Fatal("expected timestampless imported record to skip existing local progress")
	}
}

func TestShouldWriteImportedProgressComparesStableSourceFreshness(t *testing.T) {
	t.Parallel()

	local := &localProgressRow{UpdatedAt: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)}
	older := Record{UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	newer := Record{UpdatedAt: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)}

	if shouldWriteImportedProgress(older, local) {
		t.Fatal("expected older imported record to skip existing local progress")
	}
	if !shouldWriteImportedProgress(newer, local) {
		t.Fatal("expected newer imported record to overwrite existing local progress")
	}
}
