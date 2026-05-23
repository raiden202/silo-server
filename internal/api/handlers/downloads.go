package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	apimw "github.com/Silo-Server/silo-server/internal/api/middleware"
	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/download"
)

// DownloadService is the interface that the download handler depends on.
type DownloadService interface {
	CreateQueued(ctx context.Context, userID int, req download.CreateRequest, filter catalog.AccessFilter) (*download.Download, error)
	CreateQueuedBatch(ctx context.Context, userID int, seriesContentID string, filter catalog.AccessFilter) ([]*download.Download, string, error)
	ServeDirect(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int, fileID int, filter catalog.AccessFilter) error
	ServeFile(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int, downloadID string) error
	List(ctx context.Context, userID int) ([]*download.Download, error)
	Cancel(ctx context.Context, userID int, downloadID string) error
}

// DownloadHandler handles download endpoints.
type DownloadHandler struct {
	svc DownloadService
}

// NewDownloadHandler creates a new DownloadHandler.
func NewDownloadHandler(svc DownloadService) *DownloadHandler {
	return &DownloadHandler{svc: svc}
}

// downloadRequest represents the JSON body for POST /downloads.
type downloadRequest struct {
	ContentID string `json:"content_id"`
	EpisodeID string `json:"episode_id,omitempty"`
	FileID    int    `json:"file_id,omitempty"`
	Series    bool   `json:"series,omitempty"` // if true, downloads all episodes
}

// downloadResponse represents a download entry in API responses.
type downloadResponse struct {
	ID          string  `json:"id"`
	ContentID   string  `json:"content_id"`
	EpisodeID   string  `json:"episode_id,omitempty"`
	BatchID     string  `json:"batch_id,omitempty"`
	MediaFileID int     `json:"media_file_id"`
	FileSize    int64   `json:"file_size"`
	BytesSent   int64   `json:"bytes_sent"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
}

// downloadsListResponse wraps the downloads list for JSON serialization.
type downloadsListResponse struct {
	Downloads []downloadResponse `json:"downloads"`
}

func toDownloadResponse(d *download.Download) downloadResponse {
	resp := downloadResponse{
		ID:          d.ID,
		ContentID:   d.ContentID,
		EpisodeID:   d.EpisodeID,
		BatchID:     d.BatchID,
		MediaFileID: d.MediaFileID,
		FileSize:    d.FileSize,
		BytesSent:   d.BytesSent,
		Kind:        d.Kind,
		Status:      d.Status,
		CreatedAt:   d.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if d.CompletedAt != nil {
		s := d.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		resp.CompletedAt = &s
	}
	return resp
}

// HandleCreateDownload handles POST /downloads.
func (h *DownloadHandler) HandleCreateDownload(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Downloads not configured")
		return
	}

	var req downloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Invalid request body")
		return
	}

	if req.ContentID == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "content_id is required")
		return
	}

	filter := requestAccessFilter(r)

	if req.Series {
		downloads, batchID, err := h.svc.CreateQueuedBatch(r.Context(), userID, req.ContentID, filter)
		if err != nil {
			h.writeDownloadError(w, err)
			return
		}
		responses := make([]downloadResponse, 0, len(downloads))
		for _, d := range downloads {
			resp := toDownloadResponse(d)
			resp.BatchID = batchID
			responses = append(responses, resp)
		}
		writeJSON(w, http.StatusAccepted, downloadsListResponse{Downloads: responses})
		return
	}

	dl, err := h.svc.CreateQueued(r.Context(), userID, download.CreateRequest{
		ContentID: req.ContentID,
		EpisodeID: req.EpisodeID,
		FileID:    req.FileID,
	}, filter)
	if err != nil {
		h.writeDownloadError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, toDownloadResponse(dl))
}

// HandleListDownloads handles GET /downloads.
func (h *DownloadHandler) HandleListDownloads(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Downloads not configured")
		return
	}

	downloads, err := h.svc.List(r.Context(), userID)
	if err != nil {
		slog.Error("failed to list downloads", "user_id", userID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list downloads")
		return
	}

	responses := make([]downloadResponse, 0, len(downloads))
	for _, d := range downloads {
		responses = append(responses, toDownloadResponse(d))
	}
	writeJSON(w, http.StatusOK, downloadsListResponse{Downloads: responses})
}

// HandleDeleteDownload handles DELETE /downloads/{id}.
func (h *DownloadHandler) HandleDeleteDownload(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Download ID is required")
		return
	}

	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Downloads not configured")
		return
	}

	if err := h.svc.Cancel(r.Context(), userID, id); err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Download not found")
			return
		}
		slog.Error("failed to cancel download", "download_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to cancel download")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDownloadFile handles GET /downloads/{id}/file.
func (h *DownloadHandler) HandleDownloadFile(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "Download ID is required")
		return
	}

	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Downloads not configured")
		return
	}

	if err := h.svc.ServeFile(r.Context(), w, r, userID, id); err != nil {
		if errors.Is(err, download.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Download not found")
			return
		}
		if errors.Is(err, download.ErrDownloadNotActive) {
			writeError(w, http.StatusConflict, "download_inactive", "This download is no longer active")
			return
		}
		slog.Error("failed to serve download file", "download_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to serve download")
		return
	}
}

// HandleDirectDownload handles GET /direct-download?file_id=N.
func (h *DownloadHandler) HandleDirectDownload(w http.ResponseWriter, r *http.Request) {
	userID := apimw.GetUserID(r.Context())
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
		return
	}

	if h.svc == nil {
		writeError(w, http.StatusServiceUnavailable, "unavailable", "Downloads not configured")
		return
	}

	fileIDStr := r.URL.Query().Get("file_id")
	fileID, err := strconv.Atoi(fileIDStr)
	if err != nil || fileID <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "file_id query parameter is required")
		return
	}

	filter := requestAccessFilter(r)

	if err := h.svc.ServeDirect(r.Context(), w, r, userID, fileID, filter); err != nil {
		h.writeDownloadError(w, err)
		return
	}
}

func (h *DownloadHandler) writeDownloadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, download.ErrFeatureDisabled):
		writeError(w, http.StatusForbidden, "feature_disabled", "Downloads are disabled")
	case errors.Is(err, download.ErrDownloadNotAllowed):
		writeError(w, http.StatusForbidden, "forbidden", "You are not allowed to download")
	case errors.Is(err, download.ErrConcurrentLimitReached):
		writeError(w, http.StatusTooManyRequests, "download_limit_exceeded", "Maximum concurrent downloads reached")
	case errors.Is(err, download.ErrPeriodLimitReached):
		writeError(w, http.StatusTooManyRequests, "download_quota_exceeded", "Download quota exceeded for this period")
	case errors.Is(err, catalog.ErrItemNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Media item not found")
	default:
		slog.Error("download operation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process download")
	}
}
