package jellycompat

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequireSession_SkipsRefreshWhenNoAuthService(t *testing.T) {
	now := fixedNow()
	clock := func() time.Time { return now }
	store := NewSessionStore(30*24*time.Hour, clock)

	// Session with token expiring in 3 minutes (within 5min buffer).
	// With no authService configured, refresh is skipped and the session
	// is still returned (best-effort enhancement, not hard requirement).
	_ = store.Put(Session{
		Token:                 "valid-tok",
		StreamAppUserID:       1,
		StreamAppAccessToken:  "old-access",
		StreamAppRefreshToken: "refresh-tok",
		StreamAppTokenExpiry:  now.Add(3 * time.Minute),
	})

	authn := &Authenticator{sessions: store, authService: nil}
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Emby-Token", "valid-tok")
	rec := httptest.NewRecorder()

	handler := authn.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := SessionFromContext(r.Context())
		if session == nil {
			t.Fatal("expected session in context")
		}
		if session.StreamAppAccessToken != "old-access" {
			t.Errorf("expected access token unchanged, got %s", session.StreamAppAccessToken)
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireSession_PassesThroughNonExpiringToken(t *testing.T) {
	now := fixedNow()
	clock := func() time.Time { return now }
	store := NewSessionStore(30*24*time.Hour, clock)

	// Token not expiring for 30 minutes — no refresh needed.
	_ = store.Put(Session{
		Token:                "fresh-tok",
		StreamAppUserID:      1,
		StreamAppAccessToken: "good-access",
		StreamAppTokenExpiry: now.Add(30 * time.Minute),
	})

	authn := &Authenticator{sessions: store}
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Emby-Token", "fresh-tok")
	rec := httptest.NewRecorder()

	handler := authn.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := SessionFromContext(r.Context())
		if session == nil {
			t.Fatal("expected session in context")
		}
		if session.StreamAppAccessToken != "good-access" {
			t.Errorf("expected access token unchanged, got %s", session.StreamAppAccessToken)
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireSession_NoAuthService_PassesThroughExpiredStreamAppToken(t *testing.T) {
	now := fixedNow()
	clock := func() time.Time { return now }
	store := NewSessionStore(30*24*time.Hour, clock)

	// Session with already-expired StreamApp token and no authService to refresh.
	_ = store.Put(Session{
		Token:                "expired-tok",
		StreamAppUserID:      1,
		StreamAppAccessToken: "dead-access",
		StreamAppTokenExpiry: now.Add(-1 * time.Hour), // already expired
	})

	authn := &Authenticator{sessions: store, authService: nil}
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Emby-Token", "expired-tok")
	rec := httptest.NewRecorder()

	handler := authn.RequireSession(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When authService is nil and token is expired, session should still
		// be returned (the compat session itself is valid, only the StreamApp
		// token is stale). The refresh logic is a best-effort enhancement.
		session := SessionFromContext(r.Context())
		if session == nil {
			t.Fatal("expected session in context")
		}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 (no authService = skip refresh), got %d", rec.Code)
	}
}
