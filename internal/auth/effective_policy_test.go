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
	if u.MaxStreams != 0 || u.MaxTranscodes != 0 || u.MaxProfiles != 0 {
		t.Error("limits must be zero for zero groups")
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
