package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Silo-Server/silo-server/internal/auth"
)

// DemoSettingsReader is the subset of ServerSettingsStore needed by DemoGuard.
type DemoSettingsReader interface {
	Get(ctx context.Context, key string) (string, error)
}

// DemoGuard blocks destructive mutations for non-admin users when demo mode
// is enabled (server setting "demo.enabled" = "true").
//
// Allowed: browsing, playback, favorites, watchlist, ratings, collections,
// profiles, playback progress, watched state.
//
// Blocked: API key management, downloads, history imports, subtitle downloads.
type DemoGuard struct {
	settings DemoSettingsReader
	users    auth.UserLoader // nil means no admin bypass
}

// NewDemoGuard creates a new DemoGuard.
func NewDemoGuard(settings DemoSettingsReader, users auth.UserLoader) *DemoGuard {
	return &DemoGuard{settings: settings, users: users}
}

// blockedRoute defines a method + path prefix combination that is blocked in demo mode.
type blockedRoute struct {
	methods []string
	prefix  string
}

// demoBlockedRoutes lists the route patterns blocked for non-admin users in demo mode.
var demoBlockedRoutes = []blockedRoute{
	{methods: []string{"POST", "DELETE"}, prefix: "/api/v1/api-keys"},
	{methods: []string{"POST", "DELETE"}, prefix: "/api/v1/downloads"},
	{methods: []string{"POST"}, prefix: "/api/v1/history-imports"},
	{methods: []string{"POST"}, prefix: "/api/v1/subtitles/download"},
	{methods: []string{"POST"}, prefix: "/api/v1/subtitles/upload"},
	{methods: []string{"DELETE"}, prefix: "/api/v1/subtitles/"},
}

// Guard is an HTTP middleware that enforces demo mode restrictions.
func (dg *DemoGuard) Guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read-only methods always pass.
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Check demo mode (fast path: setting not set means disabled).
		enabled, _ := dg.settings.Get(r.Context(), "demo.enabled")
		if enabled != "true" {
			next.ServeHTTP(w, r)
			return
		}

		// Check if this request matches a blocked route.
		blocked := false
		path := r.URL.Path
		for _, br := range demoBlockedRoutes {
			if !strings.HasPrefix(path, br.prefix) {
				continue
			}
			for _, m := range br.methods {
				if r.Method == m {
					blocked = true
					break
				}
			}
			if blocked {
				break
			}
		}
		if !blocked {
			next.ServeHTTP(w, r)
			return
		}

		// Admins bypass all demo restrictions. Admin status is checked against
		// the loaded user (group-derived), never against token contents.
		if dg.isAdmin(r) {
			next.ServeHTTP(w, r)
			return
		}

		writeDemoBlocked(w)
	})
}

// isAdmin reports whether the request's authenticated user currently holds
// the admin permission. It first consults the user stashed in the context by
// RequireAdmin and falls back to a fresh load.
func (dg *DemoGuard) isAdmin(r *http.Request) bool {
	if user := GetUser(r.Context()); user != nil {
		return user.Enabled && user.IsAdmin
	}
	if dg.users == nil {
		return false
	}
	claims := GetClaims(r.Context())
	if claims == nil {
		return false
	}
	user, err := dg.users.GetByID(r.Context(), claims.UserID)
	if err != nil || user == nil {
		return false
	}
	return user.Enabled && user.IsAdmin
}

func writeDemoBlocked(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"demo_restricted","message":"This action is not available in demo mode."}`))
}
