package handlers

import (
	"context"
	"net/http"

	apimw "github.com/Silo-Server/silo-server/internal/api/middleware"
	"github.com/Silo-Server/silo-server/internal/models"
)

// adminUserLoader loads a user by ID for server-side admin checks.
type adminUserLoader interface {
	GetByID(ctx context.Context, id int) (*models.User, error)
}

// isAdminRequest reports whether the request's authenticated user currently
// holds the admin permission. Admin authority is always derived from the
// freshly loaded user (group-derived policy), never from token contents.
// It first consults the user stashed by the RequireAdmin middleware and
// falls back to loading via the given loader.
func isAdminRequest(r *http.Request, loader adminUserLoader) bool {
	ctx := r.Context()
	if user := apimw.GetUser(ctx); user != nil {
		return user.Enabled && user.IsAdmin
	}
	claims := apimw.GetClaims(ctx)
	if claims == nil || loader == nil {
		return false
	}
	user, err := loader.GetByID(ctx, claims.UserID)
	if err != nil || user == nil {
		return false
	}
	return user.Enabled && user.IsAdmin
}
