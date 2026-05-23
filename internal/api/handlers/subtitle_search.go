package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"

	apimw "github.com/Silo-Server/silo-server/internal/api/middleware"
	"github.com/Silo-Server/silo-server/internal/subtitles"
	"github.com/go-chi/chi/v5"
)

// SubtitleMediaResolver looks up media metadata for subtitle search.
type SubtitleMediaResolver interface {
	GetMediaFileWithMetadata(ctx context.Context, fileID int) (*MediaFileMetadata, error)
}

// MediaFileMetadata combines file info with parent item metadata for search.
type MediaFileMetadata struct {
	FileID     int
	FilePath   string
	FileSize   int64
	FileHash   string // OSHash (16-char hex)
	Resolution string
	VideoCodec string
	AudioCodec string
	Title      string
	Year       int
	IMDbID     string
	Season     int
	Episode    int
}

// SubtitleSearchHandler handles user-facing subtitle search operations.
type SubtitleSearchHandler struct {
	manager       *subtitles.Manager
	repo          subtitles.Repository
	mediaResolver SubtitleMediaResolver
}

// NewSubtitleSearchHandler creates a new SubtitleSearchHandler.
func NewSubtitleSearchHandler(
	manager *subtitles.Manager,
	repo subtitles.Repository,
	mediaResolver SubtitleMediaResolver,
) *SubtitleSearchHandler {
	return &SubtitleSearchHandler{
		manager:       manager,
		repo:          repo,
		mediaResolver: mediaResolver,
	}
}

type searchSubtitlesRequest struct {
	MediaFileID int      `json:"media_file_id"`
	Languages   []string `json:"languages"`
}

type downloadSubtitleRequest struct {
	MediaFileID     int     `json:"media_file_id"`
	Provider        string  `json:"provider"`
	SubtitleID      string  `json:"subtitle_id"`
	Language        string  `json:"language"`
	ReleaseName     string  `json:"release_name"`
	Format          string  `json:"format"`
	Score           float64 `json:"score"`
	HearingImpaired bool    `json:"hearing_impaired"`
}

// HandleSearch handles POST /api/v1/subtitles/search
func (h *SubtitleSearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchSubtitlesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	meta, err := h.mediaResolver.GetMediaFileWithMetadata(r.Context(), req.MediaFileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metadata_error", "Failed to look up media metadata")
		return
	}
	if meta == nil {
		writeError(w, http.StatusNotFound, "not_found", "Media file not found")
		return
	}

	releaseInfo := subtitles.ParseReleaseInfo(meta.FilePath)
	searchReq := subtitles.SearchRequest{
		IMDbID:    meta.IMDbID,
		Title:     meta.Title,
		Year:      meta.Year,
		Season:    meta.Season,
		Episode:   meta.Episode,
		Languages: req.Languages,
		Filename:  filepath.Base(meta.FilePath),
		FileHash:  meta.FileHash,
		MediaInfo: &subtitles.MediaMatchInfo{
			ReleaseGroup: releaseInfo.ReleaseGroup,
			Resolution:   firstNonEmpty(meta.Resolution, releaseInfo.Resolution),
			VideoCodec:   firstNonEmpty(meta.VideoCodec, releaseInfo.VideoCodec),
			AudioCodec:   firstNonEmpty(meta.AudioCodec, releaseInfo.AudioCodec),
			Source:       releaseInfo.Source,
		},
	}

	resp, err := h.manager.Search(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search_error", "Subtitle search failed")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleDownload handles POST /api/v1/subtitles/download
func (h *SubtitleSearchHandler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	var req downloadSubtitleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	userID := apimw.GetUserID(r.Context())

	sub, err := h.manager.Download(r.Context(), subtitles.DownloadRequest{
		ProviderName:    req.Provider,
		SubtitleID:      req.SubtitleID,
		MediaFileID:     req.MediaFileID,
		UserID:          &userID,
		Language:        req.Language,
		ReleaseName:     req.ReleaseName,
		Score:           req.Score,
		HearingImpaired: req.HearingImpaired,
	})
	if err != nil {
		slog.Error("subtitle download failed", "provider", req.Provider, "subtitle_id", req.SubtitleID, "error", err)
		writeError(w, http.StatusInternalServerError, "download_error", "Failed to download subtitle")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"subtitle": sub})
}

// HandleList handles GET /api/v1/subtitles/{media_file_id}
func (h *SubtitleSearchHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	mediaFileID, err := strconv.Atoi(chi.URLParam(r, "media_file_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid media file ID")
		return
	}

	subs, err := h.repo.ListDownloadedSubtitles(r.Context(), mediaFileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_error", "Failed to list subtitles")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"subtitles": subs})
}

// HandleDelete handles DELETE /api/v1/subtitles/{id}
func (h *SubtitleSearchHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid subtitle ID")
		return
	}

	sub, err := h.repo.GetDownloadedSubtitle(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_error", "Failed to look up subtitle")
		return
	}
	if sub == nil {
		writeError(w, http.StatusNotFound, "not_found", "Subtitle not found")
		return
	}

	claims := apimw.GetClaims(r.Context())
	isAdmin := claims != nil && claims.Role == "admin"
	isOwner := sub.DownloadedBy != nil && claims != nil && *sub.DownloadedBy == claims.UserID
	if !isAdmin && !isOwner {
		writeError(w, http.StatusForbidden, "forbidden", "Not authorized to delete this subtitle")
		return
	}

	if err := h.manager.DeleteSubtitle(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_error", "Failed to delete subtitle")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
