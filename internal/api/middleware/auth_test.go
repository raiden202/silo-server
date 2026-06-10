package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Silo-Server/silo-server/internal/auth"
	"github.com/Silo-Server/silo-server/internal/models"
)

// fakeAdminUserLoader implements auth.UserLoader for unit tests.
type fakeAdminUserLoader struct {
	user *models.User
	err  error
}

func (f *fakeAdminUserLoader) GetByID(_ context.Context, _ int) (*models.User, error) {
	return f.user, f.err
}

// newTestAuthMiddleware returns an AuthMiddleware wired only with a userLoader;
// the JWT/session fields are unused by RequireAdmin.
func newTestAuthMiddleware(ul auth.UserLoader) *AuthMiddleware {
	return NewAuthMiddleware(nil, nil, nil, nil, ul)
}

// adminClaims injects a minimal Claims into the given request context.
func adminClaims(r *http.Request, userID int) *http.Request {
	ctx := SetClaims(r.Context(), &auth.Claims{UserID: userID, TokenType: auth.TokenTypeAccess})
	return r.WithContext(ctx)
}

func TestRequireAdmin(t *testing.T) {
	tests := []struct {
		name       string
		setupReq   func(*http.Request) *http.Request
		loader     auth.UserLoader
		wantStatus int
		checkUser  bool // if true, assert GetUser is non-nil inside the next handler
	}{
		{
			name:       "no claims in context returns 401",
			setupReq:   func(r *http.Request) *http.Request { return r }, // no claims injected
			loader:     &fakeAdminUserLoader{user: &models.User{ID: 1, Enabled: true, IsAdmin: true}},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "nil user loader returns 403",
			setupReq:   func(r *http.Request) *http.Request { return adminClaims(r, 1) },
			loader:     nil,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "loader returns error returns 403",
			setupReq:   func(r *http.Request) *http.Request { return adminClaims(r, 1) },
			loader:     &fakeAdminUserLoader{err: errors.New("db error")},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "loader returns nil user returns 403",
			setupReq:   func(r *http.Request) *http.Request { return adminClaims(r, 1) },
			loader:     &fakeAdminUserLoader{user: nil, err: nil},
			wantStatus: http.StatusForbidden,
		},
		{
			name:     "disabled admin user returns 403",
			setupReq: func(r *http.Request) *http.Request { return adminClaims(r, 2) },
			loader: &fakeAdminUserLoader{
				user: &models.User{ID: 2, Enabled: false, IsAdmin: true},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:     "enabled non-admin user returns 403",
			setupReq: func(r *http.Request) *http.Request { return adminClaims(r, 3) },
			loader: &fakeAdminUserLoader{
				user: &models.User{ID: 3, Enabled: true, IsAdmin: false},
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:     "enabled admin reaches next handler and user is stashed in context",
			setupReq: func(r *http.Request) *http.Request { return adminClaims(r, 4) },
			loader: &fakeAdminUserLoader{
				user: &models.User{ID: 4, Enabled: true, IsAdmin: true},
			},
			wantStatus: http.StatusNoContent,
			checkUser:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			am := newTestAuthMiddleware(tc.loader)

			var gotUser *models.User
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotUser = GetUser(r.Context())
				w.WriteHeader(http.StatusNoContent)
			})

			req := httptest.NewRequest(http.MethodGet, "/admin/something", nil)
			req = tc.setupReq(req)
			rec := httptest.NewRecorder()

			am.RequireAdmin(next).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			if tc.checkUser {
				if gotUser == nil {
					t.Fatal("GetUser returned nil inside next handler, want a non-nil user")
				}
				if gotUser.ID != tc.loader.(*fakeAdminUserLoader).user.ID {
					t.Fatalf("stashed user ID = %d, want %d", gotUser.ID, tc.loader.(*fakeAdminUserLoader).user.ID)
				}
			}
		})
	}
}
