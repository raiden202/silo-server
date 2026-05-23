package playback

import (
	"fmt"
	"os"
	"path/filepath"
)

// CleanupOrphanedTranscodeDirs removes per-session transcode directories that
// are not associated with any currently active session IDs.
func CleanupOrphanedTranscodeDirs(root string, activeSessionIDs map[string]struct{}) (int, error) {
	if root == "" {
		return 0, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read transcode dir %q: %w", root, err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if _, ok := activeSessionIDs[entry.Name()]; ok {
			continue
		}

		dir := filepath.Join(root, entry.Name())
		if err := os.RemoveAll(dir); err != nil {
			return removed, fmt.Errorf("remove orphaned transcode dir %q: %w", dir, err)
		}
		removed++
	}

	return removed, nil
}
