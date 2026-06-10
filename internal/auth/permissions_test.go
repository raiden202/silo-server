package auth

import (
	"reflect"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestNormalizePermissions_DeduplicatesAndSorts(t *testing.T) {
	got, err := NormalizePermissions([]string{
		"marker_edit",
		" metadata_curation ",
		"metadata_curation",
		"",
	})
	if err != nil {
		t.Fatalf("NormalizePermissions returned error: %v", err)
	}
	want := []string{"marker_edit", "metadata_curation"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions = %#v, want %#v", got, want)
	}
}

func TestNormalizePermissions_RejectsUnknownPermission(t *testing.T) {
	if _, err := NormalizePermissions([]string{"server_owner"}); err == nil {
		t.Fatal("expected unknown permission error")
	}
}

func TestHasEffectivePermission_AdminImpliesAssignablePermissions(t *testing.T) {
	user := &models.User{Role: "admin", IsAdmin: true, Enabled: true}
	if !HasEffectivePermission(user, PermissionMetadataCuration) {
		t.Fatal("admin should have metadata curation")
	}
	if !HasEffectivePermission(user, PermissionMarkerEdit) {
		t.Fatal("admin should have marker edit")
	}
}

func TestHasEffectivePermission_UserRequiresAssignedPermission(t *testing.T) {
	user := &models.User{Role: "user", Enabled: true}
	if HasEffectivePermission(user, PermissionMetadataCuration) {
		t.Fatal("plain user should not have metadata curation")
	}
	user.Permissions = []string{"metadata_curation"}
	if !HasEffectivePermission(user, PermissionMetadataCuration) {
		t.Fatal("assigned user should have metadata curation")
	}
}

func TestDefaultUserPermissionsIncludesMarkerEditOnly(t *testing.T) {
	got := DefaultUserPermissions()
	want := []string{"marker_edit"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default permissions = %#v, want %#v", got, want)
	}
}

func TestAdminPermissionIsAssignable(t *testing.T) {
	normalized, err := NormalizePermissions([]string{"admin"})
	if err != nil {
		t.Fatalf("admin must be an assignable permission: %v", err)
	}
	if len(normalized) != 1 || normalized[0] != "admin" {
		t.Fatalf("got %v, want [admin]", normalized)
	}
}

func TestIsAdminGrantsAllPermissions(t *testing.T) {
	u := &models.User{Enabled: true, IsAdmin: true}
	if !HasEffectivePermission(u, PermissionMarkerEdit) {
		t.Error("admin should hold marker_edit")
	}
	if !HasEffectivePermission(u, PermissionMetadataCuration) {
		t.Error("admin should hold metadata_curation")
	}
}
