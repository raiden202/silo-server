package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/diagnostics/contract"
)

const (
	MaxManifestBytes       = int64(64 * 1024)
	BundleContentType      = "application/gzip"
	diagnosticObjectPrefix = "diagnostics"

	failedUploadCompensationTimeout = 10 * time.Second
)

var (
	ErrDisabled               = errors.New("diagnostics disabled")
	ErrStorageUnavailable     = errors.New("diagnostics storage unavailable")
	ErrTooLarge               = errors.New("diagnostics upload too large")
	ErrUnsupportedSchema      = errors.New("diagnostics schema version unsupported")
	ErrDestinationMismatch    = errors.New("diagnostics destination mismatch")
	ErrStaleConsent           = errors.New("diagnostics consent notice is stale")
	ErrArchiveMismatch        = errors.New("diagnostics archive metadata mismatch")
	ErrReportStoreUnavailable = errors.New("diagnostics report store unavailable")
)

type AvailabilityStatus string

const (
	StatusAvailable          AvailabilityStatus = "available"
	StatusDisabled           AvailabilityStatus = "disabled"
	StatusStorageUnavailable AvailabilityStatus = "storage_unavailable"
)

type Status struct {
	Status                 AvailabilityStatus `json:"status"`
	ServerInstanceID       string             `json:"server_instance_id"`
	AcceptedSchemaVersions []int              `json:"accepted_schema_versions"`
	MaxBundleBytes         int64              `json:"max_bundle_bytes"`
	MaxManifestBytes       int64              `json:"max_manifest_bytes"`
	RetentionDays          int                `json:"retention_days"`
	ConsentNoticeVersion   int                `json:"consent_notice_version"`
}

type IngestResult struct {
	ReportID string `json:"report_id"`
	ShortID  string `json:"short_id"`
}

type Service struct {
	reports          ReportStore
	settings         SettingsStore
	store            ObjectStore
	profileValidator ProfileAttributionValidator
	logger           *slog.Logger
	now              func() time.Time
}

func NewService(reports ReportStore, settings SettingsStore, store ObjectStore, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		reports:  reports,
		settings: settings,
		store:    store,
		logger:   logger,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

type ProfileAttributionValidator interface {
	ProfileBelongsToUser(ctx context.Context, userID int, profileID string) (bool, error)
}

type ProfileAttributionValidatorFunc func(ctx context.Context, userID int, profileID string) (bool, error)

func (f ProfileAttributionValidatorFunc) ProfileBelongsToUser(ctx context.Context, userID int, profileID string) (bool, error) {
	return f(ctx, userID, profileID)
}

// ProfileLookup resolves whether profileID exists for userID and whether it is
// a child profile. It backs NewProfileAttributionValidator.
type ProfileLookup func(ctx context.Context, userID int, profileID string) (found bool, isChild bool, err error)

// NewProfileAttributionValidator builds a ProfileAttributionValidator that
// attributes a report to a profile only when it belongs to the user and is not
// a child profile. The client diagnostics design forbids child profiles from
// performing diagnostics actions, so a nonconforming client that sends a child
// profile's ID (via X-Profile-Id or a manifest profile_id) has that attribution
// rejected rather than recorded against the child.
func NewProfileAttributionValidator(lookup ProfileLookup) ProfileAttributionValidator {
	return ProfileAttributionValidatorFunc(func(ctx context.Context, userID int, profileID string) (bool, error) {
		found, isChild, err := lookup(ctx, userID, profileID)
		if err != nil {
			return false, err
		}
		return found && !isChild, nil
	})
}

func (s *Service) SetProfileAttributionValidator(validator ProfileAttributionValidator) {
	s.profileValidator = validator
}

func (s *Service) Status(ctx context.Context, userID int) (Status, error) {
	settings, err := s.currentSettings(ctx)
	if err != nil {
		return Status{}, err
	}
	return s.statusFromSettings(settings), nil
}

func (s *Service) Ingest(ctx context.Context, userID int, profileID *string, manifestJSON []byte, bundle io.Reader) (IngestResult, error) {
	settings, err := s.currentSettings(ctx)
	if err != nil {
		s.logRejected(ctx, "", userID, "", "", 0, "settings_error")
		return IngestResult{}, err
	}
	status := s.statusFromSettings(settings).Status
	switch status {
	case StatusDisabled:
		s.logRejected(ctx, "", userID, "", "", 0, "disabled")
		return IngestResult{}, ErrDisabled
	case StatusStorageUnavailable:
		s.logRejected(ctx, "", userID, "", "", 0, "storage_unavailable")
		return IngestResult{}, ErrStorageUnavailable
	}
	if s.reports == nil {
		s.logRejected(ctx, "", userID, "", "", 0, "report_store_unavailable")
		return IngestResult{}, ErrReportStoreUnavailable
	}
	if len(manifestJSON) > int(MaxManifestBytes) {
		s.logRejected(ctx, "", userID, "", "", int64(len(manifestJSON)), "too_large")
		return IngestResult{}, ErrTooLarge
	}
	if manifestHasUnsupportedSchema(manifestJSON) {
		s.logRejected(ctx, "", userID, "", "", int64(len(manifestJSON)), "unsupported_schema")
		return IngestResult{}, ErrUnsupportedSchema
	}

	manifest, err := contract.ValidateManifest(manifestJSON)
	if err != nil {
		s.logRejected(ctx, "", userID, "", "", int64(len(manifestJSON)), "invalid_manifest")
		return IngestResult{}, fmt.Errorf("%w: invalid manifest: %v", ErrInvalidBundle, err)
	}
	if manifest.Archive.Bytes > settings.MaxBundleBytes {
		s.logRejected(ctx, "", userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, "too_large")
		return IngestResult{}, ErrTooLarge
	}
	if manifest.Destination.ServerInstanceID != settings.ServerInstanceID {
		s.logRejected(ctx, "", userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, "destination_mismatch")
		return IngestResult{}, ErrDestinationMismatch
	}
	if manifest.Consent.NoticeVersion != settings.ConsentNoticeVersion {
		s.logRejected(ctx, "", userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, "stale_consent")
		return IngestResult{}, ErrStaleConsent
	}

	reportProfileID, err := s.validatedProfileID(ctx, userID, profileID, manifest.Report.ProfileID)
	if err != nil {
		s.logRejected(ctx, "", userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, "profile_validation_error")
		return IngestResult{}, err
	}
	crashSummary := crashSummaryFromManifest(manifest)
	reserved, err := s.reports.InsertReceiving(ctx, InsertReceivingInput{
		UserID:               userID,
		ProfileID:            reportProfileID,
		CapturedAt:           manifest.Report.CapturedAt,
		ReportType:           manifest.Report.Type,
		Platform:             manifest.Report.Platform,
		AppVersion:           manifest.Report.AppVersion,
		CrashSummary:         crashSummary,
		Manifest:             append(json.RawMessage(nil), manifestJSON...),
		PlaybackSessionIDs:   manifest.PlaybackSessionIDs,
		ExpectedBlobBytes:    manifest.Archive.Bytes,
		MaxReportsPerUserDay: settings.MaxReportsPerUserDay,
		MaxBytesPerUser:      settings.MaxBytesPerUser,
		Now:                  s.now(),
	})
	if err != nil {
		reason := "insert_failed"
		if errors.Is(err, ErrQuotaExceeded) {
			reason = "quota_exceeded"
		} else {
			s.logger.ErrorContext(ctx, "diagnostic report insert failed",
				"component", "diagnostics",
				"user_id", userID,
				"error", err,
			)
		}
		s.logRejected(ctx, "", userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, reason)
		return IngestResult{}, err
	}

	key := reportObjectKey(userID, reserved.ID)
	info, err := s.putValidatedBundle(ctx, key, bundle, settings)
	if err != nil {
		s.compensateFailedUpload(ctx, reserved.ID, key)
		rejectErr := classifyBundleUploadError(err)
		s.logger.ErrorContext(ctx, "diagnostic bundle validation failed",
			"component", "diagnostics",
			"report_id", reserved.ID,
			"user_id", userID,
			"error", err,
		)
		s.logRejected(ctx, reserved.ID, userID, manifest.Report.Platform, manifest.Report.Type, manifest.Archive.Bytes, rejectReason(rejectErr))
		return IngestResult{}, rejectErr
	}
	if !archiveMatches(manifest.Archive, info) {
		s.compensateFailedUpload(ctx, reserved.ID, key)
		s.logRejected(ctx, reserved.ID, userID, manifest.Report.Platform, manifest.Report.Type, info.CompressedBytes, "archive_mismatch")
		return IngestResult{}, ErrArchiveMismatch
	}
	if !embeddedManifestMatches(manifestJSON, info.EmbeddedManifest) {
		s.compensateFailedUpload(ctx, reserved.ID, key)
		s.logRejected(ctx, reserved.ID, userID, manifest.Report.Platform, manifest.Report.Type, info.CompressedBytes, "manifest_mismatch")
		return IngestResult{}, ErrArchiveMismatch
	}

	if err := s.reports.MarkReady(ctx, reserved.ID, BlobInfo{
		Bucket:            s.store.Bucket(),
		Key:               key,
		Bytes:             info.CompressedBytes,
		UncompressedBytes: info.UncompressedBytes,
		SHA256:            info.SHA256,
	}); err != nil {
		s.compensateFailedUpload(ctx, reserved.ID, key)
		s.logRejected(ctx, reserved.ID, userID, manifest.Report.Platform, manifest.Report.Type, info.CompressedBytes, "mark_ready_failed")
		return IngestResult{}, err
	}

	s.logger.InfoContext(ctx, "diagnostic report accepted",
		"component", "diagnostics",
		"report_id", reserved.ID,
		"user_id", userID,
		"platform", manifest.Report.Platform,
		"type", manifest.Report.Type,
		"bytes", info.CompressedBytes,
		"result", "accepted",
	)
	return IngestResult{ReportID: reserved.ID, ShortID: reserved.ShortID}, nil
}

func (s *Service) currentSettings(ctx context.Context) (Settings, error) {
	settings, err := LoadSettings(ctx, s.settings)
	if err != nil {
		return Settings{}, err
	}
	if strings.TrimSpace(settings.ServerInstanceID) != "" || s.settings == nil {
		return settings, nil
	}

	instanceID, err := ensureServerInstanceID(ctx, s.settings)
	if err != nil {
		return Settings{}, err
	}
	settings.ServerInstanceID = instanceID
	return settings, nil
}

func (s *Service) statusFromSettings(settings Settings) Status {
	availability := StatusDisabled
	if settings.UploadsEnabled {
		availability = StatusAvailable
		if s.store == nil || strings.TrimSpace(s.store.Bucket()) == "" {
			availability = StatusStorageUnavailable
		}
	}

	return Status{
		Status:                 availability,
		ServerInstanceID:       settings.ServerInstanceID,
		AcceptedSchemaVersions: []int{contract.SchemaVersion},
		MaxBundleBytes:         settings.MaxBundleBytes,
		MaxManifestBytes:       MaxManifestBytes,
		RetentionDays:          settings.RetentionDays,
		ConsentNoticeVersion:   settings.ConsentNoticeVersion,
	}
}

func (s *Service) putValidatedBundle(ctx context.Context, key string, bundle io.Reader, settings Settings) (BundleInfo, error) {
	pr, pw := io.Pipe()
	resultCh := make(chan bundleValidationResult, 1)
	go func() {
		info, err := ValidateBundle(pr, BundleLimits{
			MaxCompressedBytes:   settings.MaxBundleBytes,
			MaxUncompressedBytes: settings.MaxUncompressedBytes,
		})
		if err != nil {
			_ = pr.CloseWithError(err)
		}
		resultCh <- bundleValidationResult{info: info, err: err}
	}()

	reader := &validatingUploadReader{src: bundle, pipe: pw}
	putErr := s.store.PutStream(ctx, s.store.Bucket(), key, reader, BundleContentType)
	if putErr != nil {
		reader.abort(&bundleUploadAbortError{err: putErr})
	}
	result := <-resultCh
	if putErr != nil {
		if result.err != nil && !isBundleUploadAbortError(result.err) {
			return result.info, result.err
		}
		return BundleInfo{}, fmt.Errorf("%w: store diagnostic bundle: %w", ErrStorageUnavailable, putErr)
	}
	if result.err != nil {
		return result.info, result.err
	}
	return result.info, nil
}

func (s *Service) compensateFailedUpload(ctx context.Context, reportID, key string) {
	compensationCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failedUploadCompensationTimeout)
	defer cancel()

	if s.store != nil && key != "" {
		if err := s.store.DeleteObject(compensationCtx, s.store.Bucket(), key); err != nil {
			s.logger.WarnContext(compensationCtx, "diagnostic report blob compensation failed",
				"component", "diagnostics",
				"report_id", reportID,
				"key", key,
				"error", err,
			)
		}
	}
	if s.reports != nil && reportID != "" {
		if err := s.reports.MarkFailed(compensationCtx, reportID); err != nil {
			s.logger.WarnContext(compensationCtx, "diagnostic report failed-state compensation failed",
				"component", "diagnostics",
				"report_id", reportID,
				"error", err,
			)
		}
	}
}

func (s *Service) logRejected(ctx context.Context, reportID string, userID int, platform, reportType string, bytes int64, reason string) {
	s.logger.InfoContext(ctx, "diagnostic report rejected",
		"component", "diagnostics",
		"report_id", reportID,
		"user_id", userID,
		"platform", platform,
		"type", reportType,
		"bytes", bytes,
		"result", "rejected",
		"reason", reason,
	)
}

type bundleValidationResult struct {
	info BundleInfo
	err  error
}

type validatingUploadReader struct {
	src    io.Reader
	pipe   *io.PipeWriter
	closed bool
}

func (r *validatingUploadReader) Read(p []byte) (int, error) {
	n, readErr := r.src.Read(p)
	if n > 0 {
		if _, err := r.pipe.Write(p[:n]); err != nil {
			return n, err
		}
	}
	if readErr != nil {
		r.close(readErr)
	}
	return n, readErr
}

func (r *validatingUploadReader) abort(err error) {
	r.close(err)
}

func (r *validatingUploadReader) close(err error) {
	if r.closed {
		return
	}
	r.closed = true
	if errors.Is(err, io.EOF) {
		_ = r.pipe.Close()
		return
	}
	_ = r.pipe.CloseWithError(err)
}

func classifyBundleUploadError(err error) error {
	switch {
	case errors.Is(err, ErrCompressedTooLarge),
		errors.Is(err, ErrUncompressedTooLarge),
		errors.Is(err, ErrEntryTooLarge):
		return ErrTooLarge
	case errors.Is(err, ErrInvalidBundle),
		errors.Is(err, ErrTooManyEntries),
		errors.Is(err, ErrCompressionRatio):
		return fmt.Errorf("%w: %v", ErrInvalidBundle, err)
	default:
		return err
	}
}

func rejectReason(err error) string {
	switch {
	case errors.Is(err, ErrTooLarge):
		return "too_large"
	case errors.Is(err, ErrUnsupportedSchema):
		return "unsupported_schema"
	case errors.Is(err, ErrInvalidBundle):
		return "invalid_bundle"
	default:
		return "storage_error"
	}
}

func manifestHasUnsupportedSchema(data []byte) bool {
	var header struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return false
	}
	return header.SchemaVersion != nil && *header.SchemaVersion != contract.SchemaVersion
}

func normalizedProfileID(profileID *string, fallback string) *string {
	value := ""
	if profileID != nil {
		value = strings.TrimSpace(*profileID)
	}
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return nil
	}
	return &value
}

func (s *Service) validatedProfileID(ctx context.Context, userID int, profileID *string, fallback string) (*string, error) {
	candidate := normalizedProfileID(profileID, fallback)
	if candidate == nil {
		return nil, nil
	}
	if s.profileValidator == nil {
		return nil, nil
	}
	found, err := s.profileValidator.ProfileBelongsToUser(ctx, userID, *candidate)
	if err != nil {
		return nil, fmt.Errorf("validate diagnostic profile attribution: %w", err)
	}
	if !found {
		return nil, nil
	}
	return candidate, nil
}

func crashSummaryFromManifest(manifest contract.Manifest) *string {
	if manifest.Crash == nil {
		return nil
	}
	summary := manifest.Crash.Summary
	return &summary
}

func reportObjectKey(userID int, reportID string) string {
	return diagnosticObjectPrefix + "/" + strconv.Itoa(userID) + "/" + reportID + ".tar.gz"
}

func archiveMatches(archive contract.Archive, info BundleInfo) bool {
	return archive.Bytes == info.CompressedBytes &&
		archive.UncompressedBytes == info.UncompressedBytes &&
		strings.EqualFold(archive.SHA256, info.SHA256) &&
		slices.Equal(archive.Entries, info.Entries)
}

// embeddedManifestMatches reports whether the manifest.json embedded as the
// archive's first entry equals the received part-1 manifest with its `archive`
// object removed, per the bundle contract. Both are decoded to JSON before
// comparison so formatting or key-ordering differences don't matter.
func embeddedManifestMatches(received, embedded []byte) bool {
	receivedMap, err := decodeJSONObject(received)
	if err != nil {
		return false
	}
	delete(receivedMap, "archive")
	embeddedMap, err := decodeJSONObject(embedded)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(receivedMap, embeddedMap)
}

func decodeJSONObject(data []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}
