package jellycompat

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/models"
)

// TestHandleDownload_ServesOriginalFile verifies /Items/{id}/Download streams
// the original media file. The route backs CanDownload=true, which Infuse
// requires before it will Direct Play an item.
func TestHandleDownload_ServesOriginalFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "movie.mkv")
	content := []byte("fake media bytes")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	codec := NewResourceIDCodec()
	contentID := "movie-1"
	detail := &upstreamItemDetail{
		ContentID: contentID,
		Type:      "movie",
		Versions: []catalog.FileVersion{{
			FileID:    42,
			FilePath:  filePath,
			Container: "mkv",
			Duration:  3600,
			AddedAt:   time.Now(),
		}},
	}
	handler := &PlaybackHandler{
		codec:        codec,
		content:      &stubContentService{detail: detail},
		fileResolver: testCompatFileResolver{file: &models.MediaFile{ID: 42, FilePath: filePath}},
		users:        &fakeAPIKeyUserLoader{user: &models.User{ID: 1, Enabled: true, DownloadAllowed: true}},
	}

	rec := serveDownloadRequest(handler, codec, contentID)

	if rec.Code != 200 {
		t.Fatalf("expected status 200; got %d, body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); got != string(content) {
		t.Errorf("expected file content %q; got %q", content, got)
	}
	if cd := rec.Header().Get("Content-Disposition"); cd == "" {
		t.Error("expected Content-Disposition header on download response")
	}
}

// TestHandleDownload_ForbiddenWhenDownloadsDisabled verifies the download
// route enforces the group-derived DownloadAllowed policy from a fresh user
// load and returns 403 for restricted users.
func TestHandleDownload_ForbiddenWhenDownloadsDisabled(t *testing.T) {
	codec := NewResourceIDCodec()
	handler := &PlaybackHandler{
		codec:   codec,
		content: &stubContentService{},
		users:   &fakeAPIKeyUserLoader{user: &models.User{ID: 1, Enabled: true, DownloadAllowed: false}},
	}

	rec := serveDownloadRequest(handler, codec, "movie-1")

	if rec.Code != 403 {
		t.Fatalf("expected status 403; got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// TestHandleDownload_FailsClosedWhenUserLoadFails verifies a user-load error
// never falls back to permissive defaults.
func TestHandleDownload_FailsClosedWhenUserLoadFails(t *testing.T) {
	codec := NewResourceIDCodec()
	handler := &PlaybackHandler{
		codec:   codec,
		content: &stubContentService{},
		users:   &fakeAPIKeyUserLoader{}, // GetByID errors for every ID
	}

	rec := serveDownloadRequest(handler, codec, "movie-1")

	if rec.Code != 500 {
		t.Fatalf("expected status 500; got %d, body=%s", rec.Code, rec.Body.String())
	}
}

// serveDownloadRequest issues GET /Items/{id}/Download for user 1 against
// the handler and returns the recorded response.
func serveDownloadRequest(handler *PlaybackHandler, codec *ResourceIDCodec, contentID string) *httptest.ResponseRecorder {
	encodedID := codec.EncodeStringID(EncodedIDItem, contentID)
	req := httptest.NewRequest("GET", "/Items/"+encodedID+"/Download", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", encodedID)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx)
	ctx = context.WithValue(ctx, compatSessionKey, &Session{StreamAppUserID: 1, ProfileID: "profile-1"})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.HandleDownload(rec, req)
	return rec
}

// TestItemDetail_AdvertisesCanDownload guards against regressing the
// CanDownload flag: Infuse refuses Direct Play (Static=true streaming) of
// items it believes it cannot download, so playable items must advertise it.
func TestItemDetail_AdvertisesCanDownload(t *testing.T) {
	m := newMapper(NewResourceIDCodec(), nil)
	detail := upstreamItemDetail{
		ContentID: "movie-1",
		Type:      "movie",
		Versions: []catalog.FileVersion{{
			FileID:    42,
			Container: "mkv",
			Duration:  3600,
			AddedAt:   time.Now(),
		}},
	}
	dto := m.itemFromDetailWithFields(detail, false, nil, nil)
	if !dto.CanDownload {
		t.Error("playable item detail must advertise CanDownload=true; Infuse requires it for Direct Play")
	}
}
