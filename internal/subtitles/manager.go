// internal/subtitles/manager.go
package subtitles

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager orchestrates subtitle search and download across providers.
type Manager struct {
	mu        sync.RWMutex
	providers map[string]Provider
	repo      Repository
	s3        S3Client
	s3Bucket  string
}

// NewManager creates a new subtitle manager.
func NewManager(repo Repository, s3 S3Client, s3Bucket string) *Manager {
	return &Manager{
		providers: make(map[string]Provider),
		repo:      repo,
		s3:        s3,
		s3Bucket:  s3Bucket,
	}
}

// RegisterProvider adds or replaces a provider.
func (m *Manager) RegisterProvider(p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[p.Name()] = p
}

// RemoveProvider removes a provider by name.
func (m *Manager) RemoveProvider(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.providers, name)
}

// Search fans out to all registered providers concurrently.
func (m *Manager) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	m.mu.RLock()
	providers := make([]Provider, 0, len(m.providers))
	for _, p := range m.providers {
		providers = append(providers, p)
	}
	m.mu.RUnlock()

	if len(providers) == 0 {
		return &SearchResponse{}, nil
	}

	type providerResult struct {
		results []SubtitleResult
		warning string
	}

	ch := make(chan providerResult, len(providers))
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		go func(prov Provider) {
			defer wg.Done()

			timeout := 20 * time.Second
			if prov.Name() == "subsource" {
				timeout = 30 * time.Second
			}
			pctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			results, err := prov.Search(pctx, req)
			if err != nil {
				ch <- providerResult{warning: fmt.Sprintf("%s: %v", prov.Name(), err)}
				return
			}
			ch <- providerResult{results: results}
		}(p)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	resp := &SearchResponse{}
	for pr := range ch {
		if pr.warning != "" {
			resp.Warnings = append(resp.Warnings, pr.warning)
			continue
		}
		for i := range pr.results {
			pr.results[i].Score = ScoreResult(pr.results[i], req)
		}
		resp.Results = append(resp.Results, pr.results...)
	}

	sort.Slice(resp.Results, func(i, j int) bool {
		return resp.Results[i].Score > resp.Results[j].Score
	})

	return resp, nil
}

// DownloadRequest contains metadata from the search result the user selected.
type DownloadRequest struct {
	ProviderName    string
	SubtitleID      string
	MediaFileID     int
	UserID          *int
	Language        string
	ReleaseName     string
	Score           float64
	HearingImpaired bool
}

// Download fetches a subtitle from a provider and stores it in S3.
func (m *Manager) Download(ctx context.Context, req DownloadRequest) (*DownloadedSubtitle, error) {
	m.mu.RLock()
	prov, ok := m.providers[req.ProviderName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", req.ProviderName)
	}

	data, format, err := prov.Download(ctx, req.SubtitleID)
	if err != nil {
		return nil, fmt.Errorf("download from %s: %w", req.ProviderName, err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(data))[:8]
	s3Key := fmt.Sprintf("subtitles/%d/%s_%s_%s.%s", req.MediaFileID, req.Language, req.ProviderName, hash, format)

	// Check for duplicate
	existing, err := m.repo.GetDownloadedSubtitleByS3Key(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("check duplicate: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	if err := m.s3.PutObject(ctx, m.s3Bucket, s3Key, data); err != nil {
		return nil, fmt.Errorf("upload to s3: %w", err)
	}

	sub := &DownloadedSubtitle{
		MediaFileID:     req.MediaFileID,
		Provider:        req.ProviderName,
		Language:        req.Language,
		Format:          format,
		ReleaseName:     req.ReleaseName,
		S3Key:           s3Key,
		Score:           req.Score,
		HearingImpaired: req.HearingImpaired,
		DownloadedBy:    req.UserID,
	}
	if err := m.repo.InsertDownloadedSubtitle(ctx, sub); err != nil {
		_ = m.s3.DeleteObject(ctx, m.s3Bucket, s3Key)
		return nil, fmt.Errorf("insert subtitle record: %w", err)
	}

	return sub, nil
}

// DeleteSubtitle removes a downloaded subtitle from both DB and S3.
func (m *Manager) DeleteSubtitle(ctx context.Context, id int) error {
	sub, err := m.repo.DeleteDownloadedSubtitle(ctx, id)
	if err != nil {
		return fmt.Errorf("delete subtitle record: %w", err)
	}
	if sub == nil {
		return nil
	}
	_ = m.s3.DeleteObject(ctx, m.s3Bucket, sub.S3Key)
	return nil
}
