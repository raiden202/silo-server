package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/playback"
)

type hookedSessionManager struct {
	*playback.SessionManager
	beginTransportHook func()
}

type errStreamFileResolver struct {
	err error
}

func (r errStreamFileResolver) GetByID(context.Context, int) (*models.MediaFile, error) {
	return nil, r.err
}

func (m *hookedSessionManager) BeginTransport(sessionID string) error {
	if m.beginTransportHook != nil {
		m.beginTransportHook()
	}
	return m.SessionManager.BeginTransport(sessionID)
}

func TestHandleStream_AbortsSessionWhenDirectPlayFileDisappearsAfterPreflight(t *testing.T) {
	filePath := writePlaybackTestMediaFile(t, "movie.mkv")
	file := &models.MediaFile{
		ID:        42,
		ContentID: "movie-1",
		FilePath:  filePath,
		Duration:  3600,
	}
	baseMgr := playback.NewSessionManager(0, 0)
	session, err := baseMgr.StartSession(1, "profile-1", 42, playback.PlayDirect, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	adminStore := &recordingPlaybackAdminStore{}
	syncer := &recordingSessionSyncer{}
	marker := &recordingMissingMarker{}
	sessionMgr := &hookedSessionManager{
		SessionManager: baseMgr,
		beginTransportHook: func() {
			_ = os.Remove(filePath)
		},
	}
	handler := NewStreamHandler(sessionMgr, testPlaybackFileResolver{file: file})
	handler.AdminStore = adminStore
	handler.SessionSyncer = syncer
	handler.MissingMarker = marker

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+session.ID, nil)
	req = req.WithContext(newAuthorizedPlaybackContext())
	req = withPlaybackRouteParam(req, "session_id", session.ID)

	rr := httptest.NewRecorder()
	handler.HandleStream(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := baseMgr.GetSession(session.ID); !errors.Is(err, playback.ErrSessionNotFound) {
		t.Fatalf("GetSession error = %v, want %v", err, playback.ErrSessionNotFound)
	}
	if len(marker.ids) != 1 || marker.ids[0] != 42 {
		t.Fatalf("marked ids = %v, want [42]", marker.ids)
	}
	if len(adminStore.deleted) != 1 || adminStore.deleted[0] != session.ID {
		t.Fatalf("deleted sessions = %v, want [%s]", adminStore.deleted, session.ID)
	}
	if len(adminStore.history) != 0 {
		t.Fatalf("history entries = %d, want 0", len(adminStore.history))
	}
	if syncer.calls == 0 {
		t.Fatal("expected session sync after abort")
	}
}

func TestHandleStream_KeepsSessionWhenLookupFailsForNonMissingReason(t *testing.T) {
	baseMgr := playback.NewSessionManager(0, 0)
	session, err := baseMgr.StartSession(1, "profile-1", 42, playback.PlayDirect, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	adminStore := &recordingPlaybackAdminStore{}
	syncer := &recordingSessionSyncer{}
	handler := NewStreamHandler(baseMgr, errStreamFileResolver{err: errors.New("db unavailable")})
	handler.AdminStore = adminStore
	handler.SessionSyncer = syncer

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream/"+session.ID, nil)
	req = req.WithContext(newAuthorizedPlaybackContext())
	req = withPlaybackRouteParam(req, "session_id", session.ID)

	rr := httptest.NewRecorder()
	handler.HandleStream(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if _, err := baseMgr.GetSession(session.ID); err != nil {
		t.Fatalf("GetSession error = %v, want live session", err)
	}
	if len(adminStore.deleted) != 0 {
		t.Fatalf("deleted sessions = %v, want none", adminStore.deleted)
	}
	if syncer.calls != 0 {
		t.Fatalf("sync calls = %d, want 0", syncer.calls)
	}
}

func TestHandleTransportStartFailure_KeepsSessionForNonMissingError(t *testing.T) {
	filePath := writePlaybackTestMediaFile(t, "movie.mkv")
	file := &models.MediaFile{
		ID:        42,
		ContentID: "movie-1",
		FilePath:  filePath,
		Duration:  3600,
	}
	baseMgr := playback.NewSessionManager(0, 0)
	session, err := baseMgr.StartSession(1, "profile-1", 42, playback.PlayDirect, false)
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	adminStore := &recordingPlaybackAdminStore{}
	syncer := &recordingSessionSyncer{}
	handler := NewStreamHandler(baseMgr, testPlaybackFileResolver{file: file})
	handler.AdminStore = adminStore
	handler.SessionSyncer = syncer

	handler.handleTransportStartFailure(context.Background(), session, file, errors.New("ffmpeg unavailable"))

	if _, err := baseMgr.GetSession(session.ID); err != nil {
		t.Fatalf("GetSession error = %v, want live session", err)
	}
	if len(adminStore.deleted) != 0 {
		t.Fatalf("deleted sessions = %v, want none", adminStore.deleted)
	}
	if syncer.calls != 0 {
		t.Fatalf("sync calls = %d, want 0", syncer.calls)
	}
}
