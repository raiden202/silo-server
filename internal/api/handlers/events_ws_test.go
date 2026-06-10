package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	apimw "github.com/Silo-Server/silo-server/internal/api/middleware"
	"github.com/Silo-Server/silo-server/internal/auth"
	evt "github.com/Silo-Server/silo-server/internal/events"
	"github.com/Silo-Server/silo-server/internal/models"
)

func TestRevalidateViewer(t *testing.T) {
	cases := []struct {
		name      string
		user      *models.User
		wasAdmin  bool
		wantClose bool
	}{
		{
			name:      "user deleted",
			user:      nil,
			wasAdmin:  false,
			wantClose: true,
		},
		{
			name:      "user disabled",
			user:      &models.User{ID: 1, Enabled: false},
			wasAdmin:  false,
			wantClose: true,
		},
		{
			name:      "admin disabled keeps no access even with admin flag",
			user:      &models.User{ID: 1, Enabled: false, IsAdmin: true},
			wasAdmin:  true,
			wantClose: true,
		},
		{
			name:      "admin demoted",
			user:      &models.User{ID: 1, Enabled: true, IsAdmin: false},
			wasAdmin:  true,
			wantClose: true,
		},
		{
			name:      "viewer promoted to admin",
			user:      &models.User{ID: 1, Enabled: true, IsAdmin: true},
			wasAdmin:  false,
			wantClose: true,
		},
		{
			name:      "enabled non-admin unchanged",
			user:      &models.User{ID: 1, Enabled: true, IsAdmin: false},
			wasAdmin:  false,
			wantClose: false,
		},
		{
			name:      "enabled admin unchanged",
			user:      &models.User{ID: 1, Enabled: true, IsAdmin: true},
			wasAdmin:  true,
			wantClose: false,
		},
		{
			// Policy revision bumps (group/library edits) do not affect event
			// authorization, which depends only on admin status and user ID.
			name:      "policy revision change without admin change",
			user:      &models.User{ID: 1, Enabled: true, IsAdmin: false, AccessPolicyRevision: 42},
			wasAdmin:  false,
			wantClose: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			closeConn, reason := revalidateViewer(tc.user, tc.wasAdmin)
			if closeConn != tc.wantClose {
				t.Fatalf("revalidateViewer() close = %v, want %v", closeConn, tc.wantClose)
			}
			if closeConn && reason == "" {
				t.Fatal("expected a close reason")
			}
			if !closeConn && reason != "" {
				t.Fatalf("expected empty reason for kept connection, got %q", reason)
			}
		})
	}
}

// mutableUserLoader is a swappable auth.UserLoader for revalidation tests.
type mutableUserLoader struct {
	mu   sync.Mutex
	user *models.User
	err  error
}

func (l *mutableUserLoader) GetByID(_ context.Context, id int) (*models.User, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return nil, l.err
	}
	if l.user == nil || l.user.ID != id {
		return nil, auth.ErrNotFound
	}
	userCopy := *l.user
	return &userCopy, nil
}

func (l *mutableUserLoader) set(user *models.User, err error) {
	l.mu.Lock()
	l.user = user
	l.err = err
	l.mu.Unlock()
}

// TestHandleEventsWebSocket_ClosesWhenUserDisabledMidConnection covers the
// revalidation ticker end to end: an open connection is closed by the server
// once the periodic user re-load reports the account disabled.
func TestHandleEventsWebSocket_ClosesWhenUserDisabledMidConnection(t *testing.T) {
	loader := &mutableUserLoader{user: &models.User{ID: 1, Enabled: true}}
	handler := NewEventsHandler(evt.NewHub("test", nil), nil, nil, nil, nil, nil, nil, loader)
	handler.revalidateInterval = 20 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := apimw.SetClaims(r.Context(), &auth.Claims{UserID: 1, TokenType: auth.TokenTypeAccess})
		handler.HandleWebSocket(w, r.WithContext(ctx))
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("Dial websocket: %v", err)
	}
	defer conn.Close()

	// Complete the handshake: hello, subscribe, subscribed + snapshot.
	var hello evt.EventsHelloMessage
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatalf("read hello: %v", err)
	}
	if err := conn.WriteJSON(evt.EventsSubscribeMessage{
		Type:     "subscribe",
		Channels: []evt.EventChannel{evt.ChannelCatalog},
	}); err != nil {
		t.Fatalf("write subscribe: %v", err)
	}

	// The account is disabled while the connection is open; the next
	// revalidation tick must close it with a normal close frame.
	loader.set(&models.User{ID: 1, Enabled: false}, nil)

	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		var frame json.RawMessage
		if err := conn.ReadJSON(&frame); err != nil {
			var closeErr *websocket.CloseError
			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				return // server closed the connection as required
			}
			t.Fatalf("expected normal close, got %v (%T %v)", err, err, closeErr)
		}
		// Drain subscribed/snapshot frames until the close arrives.
	}
}
