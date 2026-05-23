package download

import (
	"errors"
	"time"
)

// Download status constants.
const (
	StatusQueued      = "queued"
	StatusDownloading = "downloading"
	StatusCompleted   = "completed"
	StatusFailed      = "failed"
	StatusCancelled   = "cancelled"
)

// Download kind constants.
const (
	KindDirect = "direct"
	KindQueued = "queued"
)

// Sentinel errors.
var (
	ErrNotFound               = errors.New("download not found")
	ErrDownloadNotAllowed     = errors.New("user is not allowed to download")
	ErrFeatureDisabled        = errors.New("downloads are disabled")
	ErrConcurrentLimitReached = errors.New("concurrent download limit reached")
	ErrPeriodLimitReached     = errors.New("download period limit reached")
	ErrDownloadNotActive      = errors.New("download is not in an active state")
	ErrStatusConflict         = errors.New("download status transition conflict")
)

// Download represents a row in the downloads table.
type Download struct {
	ID           string
	UserID       int
	MediaFileID  int
	ContentID    string
	EpisodeID    string
	BatchID      string
	Kind         string // direct or queued
	Status       string // queued, downloading, completed, failed, cancelled
	FileSize     int64
	BytesSent    int64
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}
