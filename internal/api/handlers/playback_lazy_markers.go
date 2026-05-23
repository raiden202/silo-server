package handlers

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/markers"
	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/playback"
)

const playbackLazyMarkerTimeout = 10 * time.Minute

type PlaybackIntroEligibilityChecker interface {
	IntroDetectionEligibleForPlayback(ctx context.Context, fileID int) (bool, error)
}

type PlaybackMarkerUpdateNotifier interface {
	MarkersUpdated(ctx context.Context, file *models.MediaFile)
}

func (h *PlaybackHandler) maybeQueueLazyPlaybackMarkers(
	ctx context.Context,
	session *playback.Session,
	file *models.MediaFile,
) {
	if h == nil || session == nil || file == nil || file.ID <= 0 {
		return
	}
	if file.IntroStart != nil && file.IntroEnd != nil {
		return
	}
	if strings.TrimSpace(file.EpisodeID) == "" || file.MediaFolderID <= 0 {
		return
	}
	if h.SettingsRepo == nil || h.IntroRepository == nil {
		return
	}

	lazy, err := h.SettingsRepo.Get(ctx, markers.SettingLazyPlayback)
	if err != nil {
		slog.Warn("playback lazy markers: load lazy setting failed",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"error", err)
		return
	}
	if strings.ToLower(strings.TrimSpace(lazy)) != "true" {
		return
	}

	mode := markers.ModeLocal
	rawMode, err := h.SettingsRepo.Get(ctx, markers.SettingMode)
	if err != nil {
		slog.Warn("playback lazy markers: load marker mode failed",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"error", err)
		return
	}
	mode = markers.NormalizeMode(rawMode)
	if mode == markers.ModeOff {
		slog.Debug("playback lazy markers: skipped; marker mode is off",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID)
		return
	}

	eligible, err := h.IntroRepository.IntroDetectionEligibleForPlayback(ctx, file.ID)
	if err != nil {
		slog.Warn("playback lazy markers: eligibility check failed",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"mode", mode,
			"error", err)
		return
	}
	if !eligible {
		return
	}

	hasOnline := h.hasOnlineMarkerProviders()
	shouldRunLocal := markers.ShouldRunLocal(mode)
	if mode == markers.ModeOnline && !hasOnline {
		slog.Debug("playback lazy markers: skipped; online-only mode has no providers",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID)
		return
	}
	if !shouldRunLocal && mode != markers.ModeOnline {
		slog.Debug("playback lazy markers: skipped; marker mode does not allow playback detection",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"mode", mode)
		return
	}
	if shouldRunLocal && h.IntroAnalyzer == nil {
		slog.Warn("playback lazy markers: local analyzer unavailable",
			"session_id", session.ID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"mode", mode)
		return
	}

	if _, loaded := h.MarkerLazyInFlight.LoadOrStore(file.ID, struct{}{}); loaded {
		return
	}

	sessionID := session.ID
	fileSnapshot := *file
	slog.Info("playback lazy markers: queued",
		"session_id", sessionID,
		"file_id", file.ID,
		"episode_id", file.EpisodeID,
		"mode", mode)
	go h.runLazyPlaybackMarkers(sessionID, &fileSnapshot, mode)
}

func (h *PlaybackHandler) runLazyPlaybackMarkers(sessionID string, file *models.MediaFile, mode markers.Mode) {
	if file == nil {
		return
	}
	defer h.MarkerLazyInFlight.Delete(file.ID)

	base := h.MarkerLazyContext
	if base == nil {
		base = context.Background()
	}
	ctx, cancel := context.WithTimeout(base, playbackLazyMarkerTimeout)
	defer cancel()

	slog.Info("playback lazy markers: started",
		"session_id", sessionID,
		"file_id", file.ID,
		"episode_id", file.EpisodeID,
		"mode", mode)

	hasOnline := h.hasOnlineMarkerProviders()
	if (mode == markers.ModeOnline || mode == markers.ModeBoth) && hasOnline {
		wrote, err := h.fetchOnlineMarkersForPlayback(ctx, file)
		if err != nil {
			slog.Warn("playback lazy markers: online fetch failed",
				"session_id", sessionID,
				"file_id", file.ID,
				"episode_id", file.EpisodeID,
				"mode", mode,
				"error", err)
		}
		if wrote {
			if refreshed := h.reloadPlaybackMarkerFile(ctx, file.ID); hasIntroMarker(refreshed) {
				h.notifyPlaybackMarkers(ctx, sessionID, refreshed, mode)
				return
			}
		}
	}

	refreshed := h.reloadPlaybackMarkerFile(ctx, file.ID)
	if hasIntroMarker(refreshed) {
		h.notifyPlaybackMarkers(ctx, sessionID, refreshed, mode)
		return
	}

	if markers.ShouldRunLocal(mode) {
		if h.IntroAnalyzer == nil {
			slog.Warn("playback lazy markers: local analyzer unavailable",
				"session_id", sessionID,
				"file_id", file.ID,
				"episode_id", file.EpisodeID,
				"mode", mode)
			return
		}
		slog.Info("playback lazy markers: local analyzer started",
			"session_id", sessionID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"mode", mode)
		summary, err := h.IntroAnalyzer.AnalyzeEpisode(ctx, file.EpisodeID)
		if err != nil {
			slog.Warn("playback lazy markers: local analyzer failed",
				"session_id", sessionID,
				"file_id", file.ID,
				"episode_id", file.EpisodeID,
				"mode", mode,
				"error", err)
			return
		}
		slog.Info("playback lazy markers: local analyzer finished",
			"session_id", sessionID,
			"file_id", file.ID,
			"episode_id", file.EpisodeID,
			"mode", mode,
			"files_considered", summary.FilesConsidered,
			"season_groups_considered", summary.SeasonGroupsConsidered,
			"chapter_markers_written", summary.ChapterMarkersWritten,
			"chromaprint_markers_written", summary.ChromaprintMarkersWritten,
			"fingerprint_cache_hits", summary.FingerprintCacheHits,
			"fingerprints_computed", summary.FingerprintsComputed,
			"errors", len(summary.Errors))

		refreshed = h.reloadPlaybackMarkerFile(ctx, file.ID)
		if hasIntroMarker(refreshed) {
			h.notifyPlaybackMarkers(ctx, sessionID, refreshed, mode)
		}
	}
}

func (h *PlaybackHandler) hasOnlineMarkerProviders() bool {
	return h != nil && h.MarkerRegistry != nil && len(h.MarkerRegistry.Providers()) > 0
}

func (h *PlaybackHandler) fetchOnlineMarkersForPlayback(ctx context.Context, file *models.MediaFile) (bool, error) {
	if h == nil || file == nil || !h.hasOnlineMarkerProviders() {
		return false, nil
	}

	// The provider abstraction exists, but this repo does not yet wire durable
	// external IDs into playback file state. Keep online playback fetch as a
	// structured no-op until a marker provider contract can be supplied safely.
	slog.Debug("playback lazy markers: online fetch skipped; provider request identity unavailable",
		"file_id", file.ID,
		"episode_id", file.EpisodeID,
		"season_number", file.SeasonNumber,
		"episode_number", file.EpisodeNumber)
	return false, nil
}

func (h *PlaybackHandler) reloadPlaybackMarkerFile(ctx context.Context, fileID int) *models.MediaFile {
	if h == nil || h.fileResolver == nil || fileID <= 0 {
		return nil
	}
	refreshed, err := h.fileResolver.GetByID(ctx, fileID)
	if err != nil {
		slog.Warn("playback lazy markers: reload file failed", "file_id", fileID, "error", err)
		return nil
	}
	return refreshed
}

func (h *PlaybackHandler) notifyPlaybackMarkers(
	ctx context.Context,
	sessionID string,
	file *models.MediaFile,
	mode markers.Mode,
) {
	if h == nil || h.MarkerUpdateNotifier == nil || file == nil {
		return
	}
	h.MarkerUpdateNotifier.MarkersUpdated(ctx, file)
	slog.Info("playback lazy markers: emitted marker update",
		"session_id", sessionID,
		"file_id", file.ID,
		"episode_id", file.EpisodeID,
		"mode", mode)
}

func hasIntroMarker(file *models.MediaFile) bool {
	return file != nil && file.IntroStart != nil && file.IntroEnd != nil
}
