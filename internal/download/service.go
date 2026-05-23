package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Silo-Server/silo-server/internal/catalog"
	"github.com/Silo-Server/silo-server/internal/config"
	"github.com/Silo-Server/silo-server/internal/idgen"
	"github.com/Silo-Server/silo-server/internal/models"
	"github.com/Silo-Server/silo-server/internal/playback"
)

// FileResolver looks up media files by various keys.
type FileResolver interface {
	GetByID(ctx context.Context, id int) (*models.MediaFile, error)
	GetByContentID(ctx context.Context, contentID string) ([]*models.MediaFile, error)
	GetByEpisodeID(ctx context.Context, episodeID string) ([]*models.MediaFile, error)
}

// ItemResolver looks up media items.
type ItemResolver interface {
	GetByID(ctx context.Context, contentID string) (*models.MediaItem, error)
}

// EpisodeResolver lists episodes for a series.
type EpisodeResolver interface {
	ListBySeries(ctx context.Context, seriesID string) ([]*models.Episode, error)
}

// UserResolver looks up users.
type UserResolver interface {
	GetByID(ctx context.Context, id int) (*models.User, error)
}

// ItemAccessChecker checks library/content-rating access.
type ItemAccessChecker interface {
	EnsureAccessible(ctx context.Context, contentID string, filter catalog.AccessFilter) error
}

// SettingsReader loads all server settings as a flat map.
type SettingsReader interface {
	GetAll(ctx context.Context) (map[string]string, error)
}

const configCacheTTL = 30 * time.Second

// Service orchestrates download permission checks, quota enforcement,
// file resolution, and file serving.
type Service struct {
	repo        *Repository
	bandwidth   *BandwidthManager
	limiter     *QuantityLimiter
	fileRepo    FileResolver
	itemRepo    ItemResolver
	episodeRepo EpisodeResolver
	userRepo    UserResolver
	itemAccess  ItemAccessChecker
	settings    SettingsReader

	cfgMu       sync.RWMutex
	cfg         config.DownloadConfig
	cfgLoadedAt time.Time
}

// NewService creates a new download service with the given dependencies.
func NewService(
	repo *Repository,
	bandwidth *BandwidthManager,
	limiter *QuantityLimiter,
	fileRepo FileResolver,
	itemRepo ItemResolver,
	episodeRepo EpisodeResolver,
	userRepo UserResolver,
	itemAccess ItemAccessChecker,
	settings SettingsReader,
	initialCfg *config.DownloadConfig,
) *Service {
	s := &Service{
		repo:        repo,
		bandwidth:   bandwidth,
		limiter:     limiter,
		fileRepo:    fileRepo,
		itemRepo:    itemRepo,
		episodeRepo: episodeRepo,
		userRepo:    userRepo,
		itemAccess:  itemAccess,
		settings:    settings,
	}
	if initialCfg != nil {
		s.cfg = *initialCfg
		s.cfgLoadedAt = time.Now()
	}
	return s
}

// loadConfig returns the current download config, refreshing from DB if stale.
func (s *Service) loadConfig(ctx context.Context) config.DownloadConfig {
	s.cfgMu.RLock()
	if time.Since(s.cfgLoadedAt) < configCacheTTL {
		cfg := s.cfg
		s.cfgMu.RUnlock()
		return cfg
	}
	s.cfgMu.RUnlock()

	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()

	// Double-check after acquiring write lock.
	if time.Since(s.cfgLoadedAt) < configCacheTTL {
		return s.cfg
	}

	if s.settings == nil {
		return s.cfg
	}

	allSettings, err := s.settings.GetAll(ctx)
	if err != nil {
		slog.Warn("failed to reload download config from DB, using cached", "error", err)
		return s.cfg
	}

	newFullCfg, err := config.LoadFromDB(allSettings)
	if err != nil {
		slog.Warn("failed to parse download config from DB, using cached", "error", err)
		return s.cfg
	}

	oldCfg := s.cfg
	s.cfg = newFullCfg.Download
	s.cfgLoadedAt = time.Now()

	// Update bandwidth manager if limits changed.
	if s.bandwidth != nil && (oldCfg.ServerBandwidthBPS != s.cfg.ServerBandwidthBPS || oldCfg.UserBandwidthBPS != s.cfg.UserBandwidthBPS) {
		s.bandwidth.Reload(s.cfg.ServerBandwidthBPS, s.cfg.UserBandwidthBPS)
		slog.Info("download bandwidth config reloaded", "server_bps", s.cfg.ServerBandwidthBPS, "user_bps", s.cfg.UserBandwidthBPS)
	}

	// Update quantity limiter if limits changed.
	if s.limiter != nil && (oldCfg.MaxConcurrentPerUser != s.cfg.MaxConcurrentPerUser || oldCfg.MaxPerPeriod != s.cfg.MaxPerPeriod || oldCfg.PeriodDuration != s.cfg.PeriodDuration) {
		s.limiter.Reload(s.cfg.MaxConcurrentPerUser, s.cfg.MaxPerPeriod, s.cfg.PeriodDuration)
		slog.Info("download quantity limits reloaded", "max_concurrent", s.cfg.MaxConcurrentPerUser, "max_per_period", s.cfg.MaxPerPeriod, "period", s.cfg.PeriodDuration)
	}

	return s.cfg
}

// CreateRequest holds the parameters for creating a download.
type CreateRequest struct {
	ContentID string
	EpisodeID string
	FileID    int
}

// CreateQueued creates a queued download for a single item (movie or episode).
func (s *Service) CreateQueued(ctx context.Context, userID int, req CreateRequest, filter catalog.AccessFilter) (*Download, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("loading user: %w", err)
	}
	if !user.DownloadAllowed {
		return nil, ErrDownloadNotAllowed
	}

	file, err := s.resolveFile(ctx, req)
	if err != nil {
		return nil, err
	}

	if err := s.itemAccess.EnsureAccessible(ctx, file.ContentID, filter); err != nil {
		return nil, err
	}

	if err := s.limiter.Check(ctx, userID, 1); err != nil {
		return nil, err
	}

	id, err := idgen.NextID()
	if err != nil {
		return nil, fmt.Errorf("generating download ID: %w", err)
	}

	now := time.Now()
	d := &Download{
		ID:          id,
		UserID:      userID,
		MediaFileID: file.ID,
		ContentID:   file.ContentID,
		EpisodeID:   file.EpisodeID,
		Kind:        KindQueued,
		Status:      StatusQueued,
		FileSize:    file.FileSize,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// CreateQueuedBatch creates download records for all episodes in a series.
// Returns the created downloads and a shared batch ID.
func (s *Service) CreateQueuedBatch(ctx context.Context, userID int, seriesContentID string, filter catalog.AccessFilter) ([]*Download, string, error) {
	if err := s.checkEnabled(ctx); err != nil {
		return nil, "", err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("loading user: %w", err)
	}
	if !user.DownloadAllowed {
		return nil, "", ErrDownloadNotAllowed
	}

	item, err := s.itemRepo.GetByID(ctx, seriesContentID)
	if err != nil {
		return nil, "", fmt.Errorf("loading series: %w", err)
	}
	if item.Type != "series" {
		return nil, "", fmt.Errorf("content_id is not a series")
	}

	if err := s.itemAccess.EnsureAccessible(ctx, seriesContentID, filter); err != nil {
		return nil, "", err
	}

	episodes, err := s.episodeRepo.ListBySeries(ctx, seriesContentID)
	if err != nil {
		return nil, "", fmt.Errorf("listing episodes: %w", err)
	}

	var downloads []*Download
	now := time.Now()

	for _, ep := range episodes {
		files, err := s.fileRepo.GetByEpisodeID(ctx, ep.ContentID)
		if err != nil {
			return nil, "", fmt.Errorf("resolving files for episode %s: %w", ep.ContentID, err)
		}
		if len(files) == 0 {
			continue // skip episodes with no files
		}
		file := pickBestFile(files)

		id, err := idgen.NextID()
		if err != nil {
			return nil, "", fmt.Errorf("generating download ID: %w", err)
		}

		downloads = append(downloads, &Download{
			ID:          id,
			UserID:      userID,
			MediaFileID: file.ID,
			ContentID:   seriesContentID,
			EpisodeID:   ep.ContentID,
			Kind:        KindQueued,
			Status:      StatusQueued,
			FileSize:    file.FileSize,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}

	if len(downloads) == 0 {
		return nil, "", fmt.Errorf("no downloadable episodes found")
	}

	if err := s.limiter.Check(ctx, userID, len(downloads)); err != nil {
		return nil, "", err
	}

	batchID, err := idgen.NextID()
	if err != nil {
		return nil, "", fmt.Errorf("generating batch ID: %w", err)
	}
	for _, d := range downloads {
		d.BatchID = batchID
	}

	if err := s.repo.CreateBatch(ctx, downloads); err != nil {
		return nil, "", err
	}
	return downloads, batchID, nil
}

// ServeDirect validates permissions and serves a file directly for browser download.
// No persistent download record is created.
func (s *Service) ServeDirect(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int, fileID int, filter catalog.AccessFilter) error {
	if err := s.checkEnabled(ctx); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("loading user: %w", err)
	}
	if !user.DownloadAllowed {
		return ErrDownloadNotAllowed
	}

	file, err := s.fileRepo.GetByID(ctx, fileID)
	if err != nil {
		return fmt.Errorf("loading media file: %w", err)
	}
	if file == nil || file.MissingSince != nil {
		return catalog.ErrItemNotFound
	}

	if err := s.itemAccess.EnsureAccessible(ctx, file.ContentID, filter); err != nil {
		return err
	}

	return s.serveFileDownload(ctx, w, r, file, userID)
}

// ServeFile serves a queued download's file, verifying ownership and current policy.
func (s *Service) ServeFile(ctx context.Context, w http.ResponseWriter, r *http.Request, userID int, downloadID string) error {
	// Re-check policy — admin may have disabled downloads or revoked permission
	// after this download was queued.
	if err := s.checkEnabled(ctx); err != nil {
		return err
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("loading user: %w", err)
	}
	if !user.DownloadAllowed {
		return ErrDownloadNotAllowed
	}

	dl, err := s.repo.GetByID(ctx, downloadID)
	if err != nil {
		return err
	}
	if dl.UserID != userID {
		return ErrNotFound // don't reveal existence
	}
	if dl.Status == StatusCancelled || dl.Status == StatusFailed {
		return fmt.Errorf("download is %s: %w", dl.Status, ErrDownloadNotActive)
	}

	file, err := s.fileRepo.GetByID(ctx, dl.MediaFileID)
	if err != nil {
		return fmt.Errorf("loading media file: %w", err)
	}
	if file == nil || file.MissingSince != nil {
		return catalog.ErrItemNotFound
	}

	// Atomically transition queued → downloading. If another request already
	// claimed this download, return a conflict error.
	if dl.Status == StatusQueued {
		if err := s.repo.TransitionStatus(ctx, dl.ID, StatusQueued, StatusDownloading, 0, nil); err != nil {
			if errors.Is(err, ErrStatusConflict) {
				return fmt.Errorf("download already in progress: %w", ErrDownloadNotActive)
			}
			slog.Warn("failed to transition download to downloading", "download_id", dl.ID, "error", err)
		}
	}

	if err := s.serveFileDownload(ctx, w, r, file, userID); err != nil {
		if updateErr := s.repo.UpdateStatus(ctx, dl.ID, StatusFailed, 0, nil); updateErr != nil {
			slog.Error("failed to mark download as failed", "download_id", dl.ID, "error", updateErr)
		}
		return err
	}

	now := time.Now()
	if err := s.repo.UpdateStatus(ctx, dl.ID, StatusCompleted, file.FileSize, &now); err != nil {
		slog.Error("failed to mark download as completed", "download_id", dl.ID, "error", err)
	}
	return nil
}

// List returns all downloads for a user.
func (s *Service) List(ctx context.Context, userID int) ([]*Download, error) {
	return s.repo.ListByUser(ctx, userID)
}

// Cancel cancels a queued/active download or deletes a completed/failed one.
func (s *Service) Cancel(ctx context.Context, userID int, downloadID string) error {
	dl, err := s.repo.GetByID(ctx, downloadID)
	if err != nil {
		return err
	}
	if dl.UserID != userID {
		return ErrNotFound
	}

	switch dl.Status {
	case StatusQueued, StatusDownloading:
		return s.repo.CancelByID(ctx, downloadID, userID)
	default:
		return s.repo.Delete(ctx, downloadID, userID)
	}
}

func (s *Service) checkEnabled(ctx context.Context) error {
	cfg := s.loadConfig(ctx)
	if !cfg.Enabled {
		return ErrFeatureDisabled
	}
	return nil
}

func (s *Service) resolveFile(ctx context.Context, req CreateRequest) (*models.MediaFile, error) {
	if req.FileID > 0 {
		file, err := s.fileRepo.GetByID(ctx, req.FileID)
		if err != nil {
			return nil, fmt.Errorf("loading media file: %w", err)
		}
		if file == nil || file.MissingSince != nil {
			return nil, catalog.ErrItemNotFound
		}
		return file, nil
	}

	var files []*models.MediaFile
	var err error
	if req.EpisodeID != "" {
		files, err = s.fileRepo.GetByEpisodeID(ctx, req.EpisodeID)
	} else {
		files, err = s.fileRepo.GetByContentID(ctx, req.ContentID)
	}
	if err != nil {
		return nil, fmt.Errorf("resolving files: %w", err)
	}
	if len(files) == 0 {
		return nil, catalog.ErrItemNotFound
	}

	return pickBestFile(files), nil
}

func (s *Service) serveFileDownload(ctx context.Context, w http.ResponseWriter, r *http.Request, file *models.MediaFile, userID int) error {
	f, err := os.Open(file.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return catalog.ErrItemNotFound
		}
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	filename := sanitizeFilename(filepath.Base(file.FilePath))
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Type", playback.MimeFromExtension(file.FilePath))

	var reader io.ReadSeeker = f
	if s.bandwidth != nil {
		reader = s.bandwidth.ThrottledReader(ctx, f, userID)
	}

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), reader)
	return nil
}

// pickBestFile selects the highest-resolution file from a list.
func pickBestFile(files []*models.MediaFile) *models.MediaFile {
	if len(files) == 1 {
		return files[0]
	}
	best := files[0]
	for _, f := range files[1:] {
		if resolutionRank(f.Resolution) > resolutionRank(best.Resolution) {
			best = f
		}
	}
	return best
}

func resolutionRank(res string) int {
	switch strings.ToLower(res) {
	case "2160p":
		return 4
	case "1080p":
		return 3
	case "720p":
		return 2
	case "480p":
		return 1
	default:
		return 0
	}
}

func sanitizeFilename(name string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', '"', '<', '>', '|', '?', '*', ':':
			return '_'
		}
		return r
	}, name)
}
