package auth

import (
	"reflect"
	"sort"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestApplyEffectivePolicyZeroGroups(t *testing.T) {
	u := &models.User{ID: 1, Enabled: true}
	ApplyEffectivePolicy(u, nil)

	if u.IsAdmin {
		t.Error("zero groups must not grant admin")
	}
	if len(u.Permissions) != 0 {
		t.Errorf("permissions = %v, want empty", u.Permissions)
	}
	if u.LibraryIDs == nil || len(u.LibraryIDs) != 0 {
		t.Errorf("library ids = %v, want empty non-nil (access to none)", u.LibraryIDs)
	}
	// Lockout comes from the empty library list; stream/transcode limits are
	// irrelevant and stay 0. MaxProfiles is floored at 1 because 0 means
	// unlimited at the profile-creation check.
	if u.MaxStreams != 0 || u.MaxTranscodes != 0 {
		t.Errorf("stream limits = %d/%d, want 0/0", u.MaxStreams, u.MaxTranscodes)
	}
	if u.MaxProfiles != 1 {
		t.Errorf("max profiles = %d, want 1 (0 would mean unlimited)", u.MaxProfiles)
	}
	if u.DownloadAllowed || u.DownloadTranscodeAllowed {
		t.Error("downloads must be denied for zero groups")
	}
}

func TestApplyEffectivePolicyUnion(t *testing.T) {
	u := &models.User{ID: 1, Enabled: true}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 10, Permissions: []string{"marker_edit"}, LibraryIDs: []int{1, 2},
			MaxStreams: 2, MaxTranscodes: 1, MaxProfiles: 3,
			MaxPlaybackQuality: "1080p", DownloadAllowed: false},
		{ID: 20, Permissions: []string{"metadata_curation"}, LibraryIDs: []int{2, 3},
			MaxStreams: 4, MaxTranscodes: 2, MaxProfiles: 1,
			MaxPlaybackQuality: "2160p", DownloadAllowed: true},
	})

	wantPerms := []string{"marker_edit", "metadata_curation"}
	sort.Strings(u.Permissions)
	if !reflect.DeepEqual(u.Permissions, wantPerms) {
		t.Errorf("permissions = %v, want %v", u.Permissions, wantPerms)
	}
	wantLibs := []int{1, 2, 3}
	sort.Ints(u.LibraryIDs)
	if !reflect.DeepEqual(u.LibraryIDs, wantLibs) {
		t.Errorf("libraries = %v, want %v", u.LibraryIDs, wantLibs)
	}
	if u.MaxStreams != 4 || u.MaxTranscodes != 2 || u.MaxProfiles != 3 {
		t.Errorf("limits = %d/%d/%d, want 4/2/3", u.MaxStreams, u.MaxTranscodes, u.MaxProfiles)
	}
	if u.MaxPlaybackQuality != "2160p" {
		t.Errorf("quality = %q, want 2160p", u.MaxPlaybackQuality)
	}
	if !u.DownloadAllowed {
		t.Error("downloads must be OR-ed")
	}
	wantGroups := []int{10, 20}
	sort.Ints(u.GroupIDs)
	if !reflect.DeepEqual(u.GroupIDs, wantGroups) {
		t.Errorf("group ids = %v, want %v", u.GroupIDs, wantGroups)
	}
	if u.IsAdmin {
		t.Error("no group granted admin")
	}
}

func TestApplyEffectivePolicyUnlimitedLimitWins(t *testing.T) {
	// 0 means unlimited at enforcement for all three limits, so a group with
	// 0 must win the most-permissive union over any finite cap.
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, MaxStreams: 5, MaxTranscodes: 5, MaxProfiles: 5},
		{ID: 2, MaxStreams: 0, MaxTranscodes: 0, MaxProfiles: 0},
	})
	if u.MaxStreams != 0 || u.MaxTranscodes != 0 || u.MaxProfiles != 0 {
		t.Errorf("limits = %d/%d/%d, want 0/0/0 (unlimited wins)",
			u.MaxStreams, u.MaxTranscodes, u.MaxProfiles)
	}

	// Order must not matter: unlimited in the first group sticks.
	u = &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 2, MaxStreams: 0, MaxTranscodes: 0, MaxProfiles: 0},
		{ID: 1, MaxStreams: 5, MaxTranscodes: 5, MaxProfiles: 5},
	})
	if u.MaxStreams != 0 || u.MaxTranscodes != 0 || u.MaxProfiles != 0 {
		t.Errorf("limits = %d/%d/%d, want 0/0/0 (unlimited wins regardless of order)",
			u.MaxStreams, u.MaxTranscodes, u.MaxProfiles)
	}
}

func TestApplyEffectivePolicyFiniteLimitsTakeMax(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, MaxStreams: 5, MaxTranscodes: 2, MaxProfiles: 4},
		{ID: 2, MaxStreams: 2, MaxTranscodes: 5, MaxProfiles: 1},
	})
	if u.MaxStreams != 5 || u.MaxTranscodes != 5 || u.MaxProfiles != 4 {
		t.Errorf("limits = %d/%d/%d, want 5/5/4 (max of finite caps)",
			u.MaxStreams, u.MaxTranscodes, u.MaxProfiles)
	}
}

func TestApplyEffectivePolicyNilLibrariesMeansAll(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, LibraryIDs: []int{1}},
		{ID: 2, LibraryIDs: nil}, // all libraries
	})
	if u.LibraryIDs != nil {
		t.Errorf("library ids = %v, want nil (unrestricted)", u.LibraryIDs)
	}
}

func TestApplyEffectivePolicyAdminShortCircuit(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, Permissions: []string{"admin"}, LibraryIDs: []int{1},
			MaxStreams: 1, MaxPlaybackQuality: "480p"},
	})
	if !u.IsAdmin {
		t.Fatal("admin permission must set IsAdmin")
	}
	if u.LibraryIDs != nil {
		t.Error("admin must be library-unrestricted")
	}
	if u.MaxPlaybackQuality != "" {
		t.Error("admin must be quality-unrestricted")
	}
	// Admin does not bypass stream/transcode/profile limits (matching the
	// pre-group behavior where admins were subject to per-user limits); the
	// merged group values stand.
	if u.MaxStreams != 1 {
		t.Errorf("max streams = %d, want 1 (admin keeps merged group limits)", u.MaxStreams)
	}
}

func TestApplyEffectivePolicyQualityUnrestrictedWins(t *testing.T) {
	u := &models.User{}
	ApplyEffectivePolicy(u, []models.Group{
		{ID: 1, MaxPlaybackQuality: "1080p"},
		{ID: 2, MaxPlaybackQuality: ""}, // unrestricted
	})
	if u.MaxPlaybackQuality != "" {
		t.Errorf("quality = %q, want unrestricted", u.MaxPlaybackQuality)
	}
}
