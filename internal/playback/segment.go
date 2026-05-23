package playback

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseSegmentNumber extracts the numeric index from a segment name like
// "seg_00042" or "seg_00042.ts". Returns an error if the name doesn't
// match the expected pattern.
func ParseSegmentNumber(name string) (int, error) {
	// Strip extension if present (e.g., ".ts").
	name = strings.TrimSuffix(name, filepath.Ext(name))

	if !strings.HasPrefix(name, "seg_") {
		return 0, fmt.Errorf("unexpected segment name format: %q", name)
	}
	numStr := strings.TrimPrefix(name, "seg_")
	if numStr == "" {
		return 0, fmt.Errorf("empty segment number in: %q", name)
	}
	return strconv.Atoi(numStr)
}
