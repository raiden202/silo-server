package auth

import (
	"sort"

	"github.com/Silo-Server/silo-server/internal/access"
	"github.com/Silo-Server/silo-server/internal/models"
)

// ApplyEffectivePolicy populates u's policy fields with the most-permissive
// union of the given groups. Groups are the only source of policy:
//
//   - permissions: set union; "admin" implies everything
//   - libraries:   union; any nil (= all) or admin makes it unrestricted
//   - limits:      0 means unlimited at enforcement, so any group with 0
//     wins; otherwise max across groups
//   - quality:     most permissive; "" (unrestricted) wins
//   - booleans:    OR
//
// Zero groups yields the empty policy: no permissions, an empty non-nil
// library list, downloads denied. Lockout is enforced by the empty library
// list (access to no content), which makes the stream/transcode limits
// irrelevant; they stay at 0. MaxProfiles is the exception: 0 means
// unlimited at the profile-creation check, which is independent of library
// access, so it is floored at 1 to keep zero-group accounts from creating
// profiles freely. The quality ceiling is moot with no accessible content
// and is left at the lowest concrete value.
func ApplyEffectivePolicy(u *models.User, groups []models.Group) {
	u.GroupIDs = make([]int, 0, len(groups))
	u.IsAdmin = false
	u.Permissions = []string{}
	u.MaxStreams, u.MaxTranscodes, u.MaxProfiles = 0, 0, 0
	u.DownloadAllowed, u.DownloadTranscodeAllowed = false, false
	u.MaxPlaybackQuality = "480p"
	u.LibraryIDs = []int{}

	if len(groups) == 0 {
		u.MaxProfiles = 1
		return
	}

	permSet := make(map[string]struct{})
	libSet := make(map[int]struct{})
	allLibraries := false

	for i, g := range groups {
		u.GroupIDs = append(u.GroupIDs, g.ID)
		for _, p := range g.Permissions {
			permSet[p] = struct{}{}
		}
		if g.LibraryIDs == nil {
			allLibraries = true
		} else {
			for _, id := range g.LibraryIDs {
				libSet[id] = struct{}{}
			}
		}
		if i == 0 {
			u.MaxStreams = g.MaxStreams
			u.MaxTranscodes = g.MaxTranscodes
			u.MaxProfiles = g.MaxProfiles
			u.MaxPlaybackQuality = g.MaxPlaybackQuality
		} else {
			u.MaxStreams = mergeLimit(u.MaxStreams, g.MaxStreams)
			u.MaxTranscodes = mergeLimit(u.MaxTranscodes, g.MaxTranscodes)
			u.MaxProfiles = mergeLimit(u.MaxProfiles, g.MaxProfiles)
			u.MaxPlaybackQuality = access.MaxQuality(u.MaxPlaybackQuality, g.MaxPlaybackQuality)
		}
		u.DownloadAllowed = u.DownloadAllowed || g.DownloadAllowed
		u.DownloadTranscodeAllowed = u.DownloadTranscodeAllowed || g.DownloadTranscodeAllowed
	}

	// Admin implies unrestricted libraries, quality, and downloads. Stream,
	// transcode, and profile limits intentionally keep the merged group
	// values: before groups, admins were subject to their per-user limits
	// (default 6 streams) with no bypass, and the playback session manager
	// still enforces whatever it is handed.
	if _, ok := permSet[string(PermissionAdmin)]; ok {
		u.IsAdmin = true
		allLibraries = true
		u.MaxPlaybackQuality = ""
		u.DownloadAllowed = true
	}

	u.Permissions = make([]string, 0, len(permSet))
	for p := range permSet {
		u.Permissions = append(u.Permissions, p)
	}
	sort.Strings(u.Permissions)

	if allLibraries {
		u.LibraryIDs = nil
	} else {
		u.LibraryIDs = make([]int, 0, len(libSet))
		for id := range libSet {
			u.LibraryIDs = append(u.LibraryIDs, id)
		}
		sort.Ints(u.LibraryIDs)
	}
}

// mergeLimit folds group limits where 0 means unlimited at enforcement
// (max_streams, max_transcodes, max_profiles), so any unlimited group wins
// the most-permissive union; otherwise the higher cap wins.
func mergeLimit(current, next int) int {
	if current == 0 || next == 0 {
		return 0
	}
	if next > current {
		return next
	}
	return current
}
