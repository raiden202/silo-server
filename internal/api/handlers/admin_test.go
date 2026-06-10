package handlers

import (
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

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
