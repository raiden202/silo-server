package diagnostics

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

const (
	KeyUploadsEnabled        = "diagnostics.uploads_enabled"
	KeyMaxBundleBytes        = "diagnostics.max_bundle_bytes"
	KeyMaxUncompressedBytes  = "diagnostics.max_uncompressed_bytes"
	KeyMaxReportsPerUserDay  = "diagnostics.max_reports_per_user_per_day"
	KeyRetentionDays         = "diagnostics.retention_days"
	KeyMaxBytesPerUser       = "diagnostics.max_bytes_per_user"
	KeyConsentNoticeVersion  = "diagnostics.consent_notice_version"
	KeyServerInstanceID      = "diagnostics.server_instance_id"
	DefaultUploadsEnabled    = false
	DefaultMaxBundleBytes    = int64(10 * 1024 * 1024)
	DefaultMaxUncompressed   = int64(64 * 1024 * 1024)
	DefaultMaxReportsPerDay  = 20
	DefaultRetentionDays     = 30
	DefaultMaxBytesPerUser   = int64(200 * 1024 * 1024)
	DefaultConsentNoticeVer  = 1
	defaultUploadsEnabledStr = "false"
	defaultMaxBundleStr      = "10485760"
	defaultMaxUncompressed   = "67108864"
	defaultMaxReportsStr     = "20"
	defaultRetentionDaysStr  = "30"
	defaultMaxBytesUserStr   = "209715200"
	defaultConsentNoticeStr  = "1"
)

// SettingsStore is the read/write surface over server_settings used by the
// diagnostics feature gate. catalog.ServerSettingsRepo satisfies it.
type SettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type Settings struct {
	UploadsEnabled       bool
	MaxBundleBytes       int64
	MaxUncompressedBytes int64
	MaxReportsPerUserDay int
	RetentionDays        int
	MaxBytesPerUser      int64
	ConsentNoticeVersion int
	ServerInstanceID     string
}

func SeedDefaults(ctx context.Context, store SettingsStore) error {
	defaults := map[string]string{
		KeyUploadsEnabled:       defaultUploadsEnabledStr,
		KeyMaxBundleBytes:       defaultMaxBundleStr,
		KeyMaxUncompressedBytes: defaultMaxUncompressed,
		KeyMaxReportsPerUserDay: defaultMaxReportsStr,
		KeyRetentionDays:        defaultRetentionDaysStr,
		KeyMaxBytesPerUser:      defaultMaxBytesUserStr,
		KeyConsentNoticeVersion: defaultConsentNoticeStr,
	}
	for key, value := range defaults {
		existing, err := store.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("seed diagnostics defaults for %s: %w", key, err)
		}
		if existing != "" {
			continue
		}
		if err := store.Set(ctx, key, value); err != nil {
			return fmt.Errorf("seed diagnostics default %s: %w", key, err)
		}
	}

	existing, err := store.Get(ctx, KeyServerInstanceID)
	if err != nil {
		return fmt.Errorf("seed diagnostics server instance id: %w", err)
	}
	if strings.TrimSpace(existing) != "" {
		return nil
	}
	instanceID, err := newServerInstanceID()
	if err != nil {
		return err
	}
	if err := store.Set(ctx, KeyServerInstanceID, instanceID); err != nil {
		return fmt.Errorf("seed diagnostics server instance id: %w", err)
	}
	return nil
}

func DefaultSettings() Settings {
	return Settings{
		UploadsEnabled:       DefaultUploadsEnabled,
		MaxBundleBytes:       DefaultMaxBundleBytes,
		MaxUncompressedBytes: DefaultMaxUncompressed,
		MaxReportsPerUserDay: DefaultMaxReportsPerDay,
		RetentionDays:        DefaultRetentionDays,
		MaxBytesPerUser:      DefaultMaxBytesPerUser,
		ConsentNoticeVersion: DefaultConsentNoticeVer,
	}
}

func LoadSettings(ctx context.Context, store SettingsStore) (Settings, error) {
	settings := DefaultSettings()
	if store == nil {
		return settings, nil
	}
	if raw, err := store.Get(ctx, KeyUploadsEnabled); err == nil && strings.TrimSpace(raw) != "" {
		if parsed, parseErr := strconv.ParseBool(strings.TrimSpace(raw)); parseErr == nil {
			settings.UploadsEnabled = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyMaxBundleBytes); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt64(raw); parsed > 0 {
			settings.MaxBundleBytes = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyMaxUncompressedBytes); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt64(raw); parsed > 0 {
			settings.MaxUncompressedBytes = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyMaxReportsPerUserDay); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt(raw); parsed > 0 {
			settings.MaxReportsPerUserDay = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyRetentionDays); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt(raw); parsed > 0 {
			settings.RetentionDays = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyMaxBytesPerUser); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt64(raw); parsed > 0 {
			settings.MaxBytesPerUser = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyConsentNoticeVersion); err == nil && strings.TrimSpace(raw) != "" {
		if parsed := parseInt(raw); parsed > 0 {
			settings.ConsentNoticeVersion = parsed
		}
	}
	if raw, err := store.Get(ctx, KeyServerInstanceID); err == nil {
		settings.ServerInstanceID = strings.TrimSpace(raw)
	}
	return settings, nil
}

func newServerInstanceID() (string, error) {
	return uuid.NewString(), nil
}

func parseInt(s string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return v
}

func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
