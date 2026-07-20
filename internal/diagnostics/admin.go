package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const ReportDownloadContentType = BundleContentType

var ErrReportNotReady = errors.New("diagnostic report is not ready")

func (s *Service) ListForAdmin(ctx context.Context, filters ListFilters) (ListResult, error) {
	if s.reports == nil {
		return ListResult{}, ErrReportStoreUnavailable
	}
	return s.reports.ListForAdmin(ctx, filters)
}

func (s *Service) GetReport(ctx context.Context, id string) (*Report, error) {
	if s.reports == nil {
		return nil, ErrReportStoreUnavailable
	}
	return s.reports.GetByID(ctx, id)
}

func (s *Service) PresignReportDownload(ctx context.Context, report *Report, expiry time.Duration) (string, error) {
	bucket, key, err := s.readyReportBlobLocation(report)
	if err != nil {
		return "", err
	}
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	return s.store.PresignGetURL(ctx, bucket, key, expiry)
}

func (s *Service) EffectiveReportDownloadTTL(requested time.Duration) time.Duration {
	if requested <= 0 {
		return requested
	}
	effectiveStore, ok := s.store.(interface {
		EffectivePresignTTL(time.Duration) time.Duration
	})
	if !ok {
		return requested
	}
	effective := effectiveStore.EffectivePresignTTL(requested)
	if effective <= 0 {
		return requested
	}
	return effective
}

func (s *Service) OpenReportDownload(ctx context.Context, report *Report) (io.ReadCloser, error) {
	bucket, key, err := s.readyReportBlobLocation(report)
	if err != nil {
		return nil, err
	}
	return s.store.GetObject(ctx, bucket, key)
}

func (s *Service) DeleteReport(ctx context.Context, id string) (*Report, error) {
	if s.reports == nil {
		return nil, ErrReportStoreUnavailable
	}
	report, err := s.reports.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := deleteReportObjects(ctx, s.store, report, s.logger); err != nil {
		return nil, err
	}
	deleted, err := s.reports.DeleteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *Service) readyReportBlobLocation(report *Report) (string, string, error) {
	if report == nil {
		return "", "", ErrNotFound
	}
	if report.State != StateReady {
		return "", "", ErrReportNotReady
	}
	if s.store == nil || strings.TrimSpace(s.store.Bucket()) == "" {
		return "", "", ErrStorageUnavailable
	}

	bucket := stringValue(report.BlobBucket)
	if bucket == "" {
		bucket = s.store.Bucket()
	}
	key := stringValue(report.BlobKey)
	if key == "" {
		return "", "", fmt.Errorf("%w: diagnostic report has no blob key", ErrStorageUnavailable)
	}
	return bucket, key, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
