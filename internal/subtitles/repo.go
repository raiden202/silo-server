// internal/subtitles/repo.go
package subtitles

import "context"

// Repository defines database operations for subtitle management.
type Repository interface {
	InsertDownloadedSubtitle(ctx context.Context, sub *DownloadedSubtitle) error
	GetDownloadedSubtitle(ctx context.Context, id int) (*DownloadedSubtitle, error)
	ListDownloadedSubtitles(ctx context.Context, mediaFileID int) ([]DownloadedSubtitle, error)
	DeleteDownloadedSubtitle(ctx context.Context, id int) (*DownloadedSubtitle, error)
	GetDownloadedSubtitleByS3Key(ctx context.Context, s3Key string) (*DownloadedSubtitle, error)

	ListProviderConfigs(ctx context.Context) ([]ProviderConfig, error)
	GetProviderConfig(ctx context.Context, providerName string) (*ProviderConfig, error)
	UpsertProviderConfig(ctx context.Context, cfg *ProviderConfig) error
}
