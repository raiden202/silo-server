package jellycompat

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/auth"
)

type sessionContextKey string

const compatSessionKey sessionContextKey = "jellycompat_session"

var mediaBrowserTokenPattern = regexp.MustCompile(`(?i)token="?([^",\s]+)"?`)

const (
	tokenRefreshBuffer  = 5 * time.Minute
	tokenRefreshTimeout = 30 * time.Second
)

// Authenticator extracts Jellyfin-style auth tokens and resolves compat sessions.
type Authenticator struct {
	sessions    *SessionStore
	authService *auth.Service
	now         func() time.Time
}

// NewAuthenticator creates a new compat authenticator.
func NewAuthenticator(sessions *SessionStore, authService *auth.Service) *Authenticator {
	return &Authenticator{sessions: sessions, authService: authService, now: time.Now}
}

// ExtractToken extracts a compat token from Jellyfin-style request auth.
// Checks Authorization, X-Emby-Authorization, X-Emby-Token,
// X-Mediabrowser-Token headers and api_key query parameter.
func ExtractToken(r *http.Request) (string, bool) {
	// Check Authorization and X-Emby-Authorization headers for Bearer or
	// MediaBrowser Token="..." formats.  The Jellyfin Kotlin SDK (used by
	// Findroid and other Android clients) sends X-Emby-Authorization.
	for _, headerName := range []string{"Authorization", "X-Emby-Authorization"} {
		if header := strings.TrimSpace(r.Header.Get(headerName)); header != "" {
			parts := strings.SplitN(header, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
				token := strings.TrimSpace(parts[1])
				if token != "" {
					return token, true
				}
			}
			if match := mediaBrowserTokenPattern.FindStringSubmatch(header); len(match) == 2 && match[1] != "" {
				return match[1], true
			}
		}
	}

	if token := strings.TrimSpace(r.Header.Get("X-Emby-Token")); token != "" {
		return token, true
	}
	if token := strings.TrimSpace(r.Header.Get("X-Mediabrowser-Token")); token != "" {
		return token, true
	}
	if token := strings.TrimSpace(r.URL.Query().Get("api_key")); token != "" {
		return token, true
	}

	return "", false
}

// RequireSession enforces a valid compat session for a route.
func (a *Authenticator) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := ExtractToken(r)
		if !ok {
			slog.Warn("jellycompat auth: no token in request",
				"path", r.URL.Path,
				"auth_header_present", r.Header.Get("Authorization") != "",
				"x_emby_auth_present", r.Header.Get("X-Emby-Authorization") != "",
			)
			writeError(w, http.StatusUnauthorized, "Unauthorized", "Missing authentication token")
			return
		}

		session, ok := a.sessions.Get(token)
		if !ok {
			slog.Warn("jellycompat auth: session not found",
				"path", r.URL.Path,
				"token_prefix", safeTokenPrefix(token),
			)
			writeError(w, http.StatusUnauthorized, "Unauthorized", "Invalid or expired authentication token")
			return
		}

		// Refresh underlying Silo tokens if they're about to expire.
		// Use a detached context so a client aborting the request mid-refresh
		// (common on flaky mobile networks) doesn't revoke the compat session.
		if a.authService != nil && !session.StreamAppTokenExpiry.IsZero() &&
			session.StreamAppTokenExpiry.Before(a.now().Add(tokenRefreshBuffer)) {
			refreshCtx, cancel := context.WithTimeout(context.Background(), tokenRefreshTimeout)
			newPair, err := a.authService.Refresh(refreshCtx, session.StreamAppRefreshToken)
			cancel()
			if err != nil {
				slog.Warn("jellycompat auth: token refresh failed, revoking session",
					"path", r.URL.Path,
					"token_prefix", safeTokenPrefix(token),
					"error", err,
				)
				a.sessions.Delete(token)
				writeError(w, http.StatusUnauthorized, "Unauthorized", "Session expired")
				return
			}
			updateErr := a.sessions.Update(token, func(s *Session) error {
				s.StreamAppAccessToken = newPair.AccessToken
				s.StreamAppRefreshToken = newPair.RefreshToken
				s.StreamAppTokenExpiry = a.now().Add(time.Duration(newPair.ExpiresIn) * time.Second)
				return nil
			})
			if updateErr != nil {
				slog.Warn("jellycompat auth: session update after refresh failed",
					"token_prefix", safeTokenPrefix(token),
					"error", updateErr,
				)
			} else {
				// Re-read the session to get the updated tokens.
				session, _ = a.sessions.Get(token)
			}
		}

		ctx := context.WithValue(r.Context(), compatSessionKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// safeTokenPrefix returns the first 8 characters of a token for logging.
func safeTokenPrefix(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}

// PlaybackSessionAuth creates middleware that falls back to playback session
// authentication for media stream endpoints where external players (e.g. libmpv)
// don't forward auth headers or query parameters.
func PlaybackSessionAuth(sessions *SessionStore, playbackStore *PlaybackSessionStore) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try standard token auth first.
			if token, ok := ExtractToken(r); ok {
				if session, sessionOK := sessions.Get(token); sessionOK {
					ctx := context.WithValue(r.Context(), compatSessionKey, session)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// Prefer the exact negotiated playback session when the client
			// includes PlaySessionId on follow-up stream requests.
			if playSessionID := firstNonEmpty(r.URL.Query().Get("PlaySessionId"), r.URL.Query().Get("PlaySessionID")); playSessionID != "" {
				if playSession, found := playbackStore.Get(playSessionID); found {
					if session, ok := sessions.Get(playSession.CompatToken); ok {
						ctx := context.WithValue(r.Context(), compatSessionKey, session)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
					writeError(w, http.StatusUnauthorized, "Unauthorized", "Session expired")
					return
				}
			}
			writeError(w, http.StatusUnauthorized, "Unauthorized", "Missing authentication token")
		})
	}
}

// SessionFromContext returns the authenticated compat session, if present.
func SessionFromContext(ctx context.Context) *Session {
	session, _ := ctx.Value(compatSessionKey).(*Session)
	return session
}
