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
//   - limits:      max across groups
//   - quality:     most permissive; "" (unrestricted) wins
//   - booleans:    OR
//
// Zero groups yields the empty policy: no permissions, an empty non-nil
// library list (access to none), zero limits, downloads denied. The quality
// ceiling is moot at zero streams and left at the lowest concrete value.
func ApplyEffectivePolicy(u *models.User, groups []models.Group) {
	u.GroupIDs = make([]int, 0, len(groups))
	u.IsAdmin = false
	u.Permissions = []string{}
	u.MaxStreams, u.MaxTranscodes, u.MaxProfiles = 0, 0, 0
	u.DownloadAllowed, u.DownloadTranscodeAllowed = false, false
	u.MaxPlaybackQuality = "480p"
	u.LibraryIDs = []int{}

	if len(groups) == 0 {
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
		if g.MaxStreams > u.MaxStreams {
			u.MaxStreams = g.MaxStreams
		}
		if g.MaxTranscodes > u.MaxTranscodes {
			u.MaxTranscodes = g.MaxTranscodes
		}
		if g.MaxProfiles > u.MaxProfiles {
			u.MaxProfiles = g.MaxProfiles
		}
		if i == 0 {
			u.MaxPlaybackQuality = g.MaxPlaybackQuality
		} else {
			u.MaxPlaybackQuality = access.MaxQuality(u.MaxPlaybackQuality, g.MaxPlaybackQuality)
		}
		u.DownloadAllowed = u.DownloadAllowed || g.DownloadAllowed
		u.DownloadTranscodeAllowed = u.DownloadTranscodeAllowed || g.DownloadTranscodeAllowed
	}

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
