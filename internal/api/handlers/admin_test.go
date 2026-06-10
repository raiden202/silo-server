package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

// fakeAdminUserRepo is a configurable in-memory UserRepository for handler
// tests.
type fakeAdminUserRepo struct {
	user      *models.User
	updateErr error
	deleteErr error
}

func (f *fakeAdminUserRepo) List(context.Context) ([]*models.User, error) { return nil, nil }

func (f *fakeAdminUserRepo) Create(context.Context, models.CreateUserInput) (*models.User, error) {
	return f.user, nil
}

func (f *fakeAdminUserRepo) Update(context.Context, int, models.UpdateUserInput) error {
	return f.updateErr
}

func (f *fakeAdminUserRepo) Delete(context.Context, int) error { return f.deleteErr }

func (f *fakeAdminUserRepo) GetByID(context.Context, int) (*models.User, error) {
	if f.user == nil {
		return nil, auth.ErrNotFound
	}
	return f.user, nil
}

// newAdminUsersTestRouter mounts the user-management routes the same way the
// API router does so tests exercise real chi URL params.
func newAdminUsersTestRouter(repo UserRepository) chi.Router {
	h := NewAdminHandler(repo, nil, nil)
	r := chi.NewRouter()
	r.Put("/admin/users/{id}", h.HandleUpdateUser)
	r.Delete("/admin/users/{id}", h.HandleDeleteUser)
	return r
}

func TestAdminUpdateUserDisableLastAdministrator(t *testing.T) {
	repo := &fakeAdminUserRepo{
		user:      &models.User{ID: 2, Enabled: true},
		updateErr: auth.ErrLastAdministrator,
	}
	rec := doGroupsRequest(t, newAdminUsersTestRouter(repo), http.MethodPut,
		"/admin/users/2", map[string]any{"enabled": false})
	assertGroupsError(t, rec, http.StatusConflict, "last_administrator")
}

func TestAdminDeleteUserLastAdministrator(t *testing.T) {
	repo := &fakeAdminUserRepo{deleteErr: auth.ErrLastAdministrator}
	rec := doGroupsRequest(t, newAdminUsersTestRouter(repo), http.MethodDelete,
		"/admin/users/2", nil)
	assertGroupsError(t, rec, http.StatusConflict, "last_administrator")
}

func TestUpdateRequiresSessionRevocation(t *testing.T) {
	enabled := true
	disabled := false
	password := "new-password"
	username := "renamed"
	newGroups := []int{1, 2}
	sameGroups := []int{2, 1, 1} // order and duplicates must not matter
	emptyGroups := []int{}

	current := &models.User{
		Enabled:  false,
		GroupIDs: []int{1, 2},
	}

	tests := []struct {
		name string
		in   models.UpdateUserInput
		want bool
	}{
		{
			name: "enabled",
			in:   models.UpdateUserInput{Enabled: &enabled},
			want: true,
		},
		{
			name: "enabled unchanged",
			in:   models.UpdateUserInput{Enabled: &disabled},
			want: false,
		},
		{
			name: "groups unchanged",
			in:   models.UpdateUserInput{GroupIDs: &sameGroups},
			want: false,
		},
		{
			name: "groups cleared",
			in:   models.UpdateUserInput{GroupIDs: &emptyGroups},
			want: true,
		},
		{
			name: "password",
			in:   models.UpdateUserInput{Password: &password},
			want: true,
		},
		{
			name: "non access fields",
			in:   models.UpdateUserInput{Username: &username},
			want: false,
		},
		{
			name: "empty update",
			in:   models.UpdateUserInput{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateRequiresSessionRevocation(current, tt.in); got != tt.want {
				t.Fatalf("updateRequiresSessionRevocation() = %v, want %v", got, tt.want)
			}
		})
	}

	memberless := &models.User{Enabled: true, GroupIDs: nil}
	t.Run("groups assigned to memberless user", func(t *testing.T) {
		if got := updateRequiresSessionRevocation(memberless, models.UpdateUserInput{GroupIDs: &newGroups}); !got {
			t.Fatalf("updateRequiresSessionRevocation() = %v, want true", got)
		}
	})

	t.Run("unknown current user falls back to may-require", func(t *testing.T) {
		if got := updateRequiresSessionRevocation(nil, models.UpdateUserInput{GroupIDs: &sameGroups}); !got {
			t.Fatalf("updateRequiresSessionRevocation() = %v, want true", got)
		}
		if got := updateRequiresSessionRevocation(nil, models.UpdateUserInput{Username: &username}); got {
			t.Fatalf("updateRequiresSessionRevocation() = %v, want false", got)
		}
	})
}
