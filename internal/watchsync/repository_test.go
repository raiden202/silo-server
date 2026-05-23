package watchsync

import (
	"strings"
	"testing"
)

func TestMediaDurationQueryUsesActiveMediaFilesPredicate(t *testing.T) {
	if !strings.Contains(mediaDurationQuery, "missing_since IS NULL") {
		t.Fatalf("media duration query must filter active files with missing_since IS NULL:\n%s", mediaDurationQuery)
	}
	if strings.Contains(mediaDurationQuery, "missing = false") {
		t.Fatalf("media duration query references removed media_files.missing column:\n%s", mediaDurationQuery)
	}
}
