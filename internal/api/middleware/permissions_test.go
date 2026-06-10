package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

type fakePermissionUserLoader struct {
	user *models.User
	err  error
}

func (f fakePermissionUserLoader) GetByID(context.Context, int) (*models.User, error) {
	return f.user, f.err
}

type fakeTargetLibraryResolver struct {
	ids []int
	err error
}

func (f fakeTargetLibraryResolver) ResolveMetadataTargetLibraryIDs(context.Context, string) ([]int, error) {
	return f.ids, f.err
}

func requestWithItemID() *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/admin/items/item-1/refresh-metadata", nil)
	ctx := SetClaims(req.Context(), &auth.Claims{UserID: 7, TokenType: auth.TokenTypeAccess})
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "item-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)
	return req.WithContext(ctx)
}

func runMetadataCurationMiddleware(user *models.User, libraryIDs []int) int {
	mw := NewPermissionMiddleware(
		fakePermissionUserLoader{user: user},
		fakeTargetLibraryResolver{ids: libraryIDs},
	)
	next := mw.RequireMetadataCurationForItem(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	next.ServeHTTP(rec, requestWithItemID())
	return rec.Code
}

func TestRequireMetadataCurationForItem_AllowsAdmin(t *testing.T) {
	code := runMetadataCurationMiddleware(&models.User{ID: 7, Enabled: true, IsAdmin: true}, nil)
	if code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
	}
}

func TestRequireMetadataCurationForItem_RejectsUserWithoutPermission(t *testing.T) {
	user := &models.User{ID: 7, Enabled: true, LibraryIDs: []int{1}, Permissions: nil}
	code := runMetadataCurationMiddleware(user, []int{1})
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", code, http.StatusForbidden)
	}
}

func TestRequireMetadataCurationForItem_AllowsUnrestrictedCurator(t *testing.T) {
	user := &models.User{ID: 7, Enabled: true, LibraryIDs: nil, Permissions: []string{"metadata_curation"}}
	code := runMetadataCurationMiddleware(user, []int{1, 2})
	if code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
	}
}

func TestRequireMetadataCurationForItem_AllowsWhenAllTargetLibrariesAreAllowed(t *testing.T) {
	user := &models.User{ID: 7, Enabled: true, LibraryIDs: []int{1, 2, 3}, Permissions: []string{"metadata_curation"}}
	code := runMetadataCurationMiddleware(user, []int{1, 3})
	if code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", code, http.StatusNoContent)
	}
}

func TestRequireMetadataCurationForItem_RejectsWhenAnyTargetLibraryIsOutsideAccess(t *testing.T) {
	user := &models.User{ID: 7, Enabled: true, LibraryIDs: []int{1}, Permissions: []string{"metadata_curation"}}
	code := runMetadataCurationMiddleware(user, []int{1, 2})
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", code, http.StatusForbidden)
	}
}

func TestRequireMetadataCurationForItem_NotFoundWhenTargetHasNoLibraries(t *testing.T) {
	user := &models.User{ID: 7, Enabled: true, LibraryIDs: nil, Permissions: []string{"metadata_curation"}}
	code := runMetadataCurationMiddleware(user, nil)
	if code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", code, http.StatusNotFound)
	}
}
