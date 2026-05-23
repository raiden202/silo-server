package metadata

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Silo-Server/silo-server/internal/cache"
	"github.com/Silo-Server/silo-server/internal/catalog"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"golang.org/x/sync/singleflight"
)

const (
	resolvedURLCacheSafetyMargin = 5 * time.Minute
	maxResolvedURLCacheTTL       = 24 * time.Hour
)

// PluginImageResolverSource provides image URL resolution for a single plugin.
type PluginImageResolverSource interface {
	ResolveImageURL(ctx context.Context, path string, variant string) (string, error)
	ResolveImageURLs(ctx context.Context, paths []string, variant string) (map[string]string, error)
}

type expiringPluginImageResolverSource interface {
	ResolveImageURLWithExpiry(ctx context.Context, path string, variant string) (catalog.ResolvedImageURL, error)
	ResolveImageURLsWithExpiry(ctx context.Context, paths []string, variant string) (map[string]catalog.ResolvedImageURL, error)
}

// PluginImageResolver resolves plugin-prefixed image paths (e.g., "metadb://images/abc/original.jpg")
// by parsing the prefix, routing to the correct plugin, and returning resolved URLs.
// It implements catalog.ImageResolver and the catalog expiry-aware resolver extension.
type PluginImageResolver struct {
	mu           sync.RWMutex
	sources      map[string]PluginImageResolverSource
	s3Presigner  s3ImagePresigner
	s3PresignTTL time.Duration
	urlCache     *cache.TTLCache[catalog.ResolvedImageURL]
	group        singleflight.Group
}

// NewPluginImageResolver creates a new resolver with no registered sources.
func NewPluginImageResolver() *PluginImageResolver {
	return &PluginImageResolver{
		sources:      make(map[string]PluginImageResolverSource),
		s3PresignTTL: 15 * time.Minute,
		urlCache:     cache.NewTTLCache[catalog.ResolvedImageURL](),
	}
}

type s3ImagePresigner interface {
	PresignGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	Bucket() string
}

// RegisterSource registers a plugin provider as a source for resolving images
// with the given plugin ID prefix.
func (r *PluginImageResolver) RegisterSource(pluginID string, source PluginImageResolverSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[pluginID] = source
}

func (r *PluginImageResolver) SetS3Presigner(presigner s3ImagePresigner, ttl time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.s3Presigner = presigner
	if ttl > 0 {
		r.s3PresignTTL = ttl
	}
}

// Close stops the resolver cache sweeper.
func (r *PluginImageResolver) Close() {
	if r.urlCache != nil {
		r.urlCache.Close()
	}
}

// ResolveImageURL resolves a single plugin-prefixed image path.
func (r *PluginImageResolver) ResolveImageURL(ctx context.Context, path string, variant string) string {
	return r.ResolveImageURLWithExpiry(ctx, path, variant).URL
}

// ResolveImageURLWithExpiry resolves a single image path and returns validity metadata when known.
func (r *PluginImageResolver) ResolveImageURLWithExpiry(ctx context.Context, path string, variant string) catalog.ResolvedImageURL {
	if path == "" {
		return catalog.ResolvedImageURL{}
	}
	resolved := r.ResolveImageURLsWithExpiry(ctx, []string{path}, variant)
	return resolved[path]
}

// ResolveImageURLs resolves multiple plugin-prefixed image paths.
func (r *PluginImageResolver) ResolveImageURLs(ctx context.Context, paths []string, variant string) map[string]string {
	resolvedWithExpiry := r.ResolveImageURLsWithExpiry(ctx, paths, variant)
	resolved := make(map[string]string, len(resolvedWithExpiry))
	for path, value := range resolvedWithExpiry {
		resolved[path] = value.URL
	}
	return resolved
}

// ResolveImageURLsWithExpiry resolves multiple image paths, caches only URLs
// with known expiry, and coalesces concurrent identical batch misses.
func (r *PluginImageResolver) ResolveImageURLsWithExpiry(ctx context.Context, paths []string, variant string) map[string]catalog.ResolvedImageURL {
	if len(paths) == 0 {
		return map[string]catalog.ResolvedImageURL{}
	}

	result := make(map[string]catalog.ResolvedImageURL, len(paths))
	grouped := make(map[string]map[string]resolveEntry)
	for _, path := range paths {
		if path == "" {
			continue
		}
		if value, ok := r.urlCache.Get(resolvedImageCacheKey(variant, path)); ok {
			result[path] = value
			continue
		}
		pluginID, barePath := parsePluginPrefix(path)
		if pluginID == "" {
			barePath = path
		}
		if grouped[pluginID] == nil {
			grouped[pluginID] = make(map[string]resolveEntry)
		}
		grouped[pluginID][path] = resolveEntry{
			barePath:     barePath,
			originalPath: path,
		}
	}
	if len(grouped) == 0 {
		return result
	}

	r.mu.RLock()
	presigner := r.s3Presigner
	s3TTL := r.s3PresignTTL
	sourcesSnapshot := make(map[string]PluginImageResolverSource, len(grouped))
	for pluginID := range grouped {
		if pluginID == "" {
			continue
		}
		if source, ok := r.sources[pluginID]; ok {
			sourcesSnapshot[pluginID] = source
		}
	}
	r.mu.RUnlock()

	for pluginID, groupedEntries := range grouped {
		entries := sortedResolveEntries(groupedEntries)
		flightKey := resolvedImageBatchFlightKey(pluginID, variant, entries)
		value, err, _ := r.group.Do(flightKey, func() (any, error) {
			if pluginID == "" {
				return r.resolveS3Batch(ctx, presigner, s3TTL, entries), nil
			}
			source, ok := sourcesSnapshot[pluginID]
			if !ok {
				slog.Warn("no image resolver registered for plugin", "plugin_id", pluginID)
				return map[string]catalog.ResolvedImageURL{}, nil
			}
			return r.resolvePluginBatch(ctx, pluginID, source, entries, variant), nil
		})
		if err != nil {
			slog.Error("image batch resolution failed", "plugin_id", pluginID, "error", err)
			continue
		}

		resolvedBatch, ok := value.(map[string]catalog.ResolvedImageURL)
		if !ok {
			continue
		}
		now := time.Now()
		for path, resolvedURL := range resolvedBatch {
			result[path] = resolvedURL
			if ttl := cacheTTLForResolvedURL(resolvedURL, now); ttl > 0 {
				r.urlCache.Set(resolvedImageCacheKey(variant, path), resolvedURL, ttl)
			}
		}
	}

	return result
}

func (r *PluginImageResolver) resolveS3Batch(
	ctx context.Context,
	presigner s3ImagePresigner,
	ttl time.Duration,
	entries []resolveEntry,
) map[string]catalog.ResolvedImageURL {
	resolved := make(map[string]catalog.ResolvedImageURL, len(entries))
	if presigner == nil {
		return resolved
	}
	expiresAt := time.Now().Add(ttl)
	for _, entry := range entries {
		url, err := presigner.PresignGetURL(ctx, presigner.Bucket(), entry.originalPath, ttl)
		if err != nil {
			slog.Error("s3 image resolution failed", "path", entry.originalPath, "error", err)
			continue
		}
		expiry := expiresAt
		resolved[entry.originalPath] = catalog.ResolvedImageURL{URL: url, ExpiresAt: &expiry}
	}
	return resolved
}

func (r *PluginImageResolver) resolvePluginBatch(
	ctx context.Context,
	pluginID string,
	source PluginImageResolverSource,
	entries []resolveEntry,
	variant string,
) map[string]catalog.ResolvedImageURL {
	barePaths := make([]string, len(entries))
	for i, entry := range entries {
		barePaths[i] = entry.barePath
	}

	var (
		resolvedByBare map[string]catalog.ResolvedImageURL
		err            error
	)
	if expiringSource, ok := source.(expiringPluginImageResolverSource); ok {
		resolvedByBare, err = expiringSource.ResolveImageURLsWithExpiry(ctx, barePaths, variant)
	} else {
		legacyURLs, legacyErr := source.ResolveImageURLs(ctx, barePaths, variant)
		err = legacyErr
		resolvedByBare = make(map[string]catalog.ResolvedImageURL, len(legacyURLs))
		for barePath, url := range legacyURLs {
			resolvedByBare[barePath] = catalog.ResolvedImageURL{URL: url}
		}
	}
	if err != nil {
		slog.Error("plugin batch image resolution failed", "plugin_id", pluginID, "error", err)
		return map[string]catalog.ResolvedImageURL{}
	}

	resolved := make(map[string]catalog.ResolvedImageURL, len(entries))
	for _, entry := range entries {
		if value, ok := resolvedByBare[entry.barePath]; ok {
			resolved[entry.originalPath] = value
		}
	}
	return resolved
}

type resolveEntry struct {
	barePath     string
	originalPath string
}

func sortedResolveEntries(entriesByOriginal map[string]resolveEntry) []resolveEntry {
	entries := make([]resolveEntry, 0, len(entriesByOriginal))
	for _, entry := range entriesByOriginal {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].originalPath < entries[j].originalPath
	})
	return entries
}

func resolvedImageCacheKey(variant, path string) string {
	return variant + "\x00" + path
}

func resolvedImageBatchFlightKey(pluginID, variant string, entries []resolveEntry) string {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.barePath
	}
	sort.Strings(paths)
	sum := sha256.Sum256([]byte(strings.Join(paths, "\x00")))
	return pluginID + "|" + variant + "|" + hex.EncodeToString(sum[:])
}

func cacheTTLForResolvedURL(value catalog.ResolvedImageURL, now time.Time) time.Duration {
	if value.URL == "" || value.ExpiresAt == nil {
		return 0
	}
	ttl := value.ExpiresAt.Sub(now) - resolvedURLCacheSafetyMargin
	if ttl <= 0 {
		return 0
	}
	if ttl > maxResolvedURLCacheTTL {
		return maxResolvedURLCacheTTL
	}
	return ttl
}

// PluginMetadataClient is the public interface for image resolution RPC calls.
type PluginMetadataClient interface {
	ResolveImageURL(ctx context.Context, req *pluginv1.ResolveImageURLRequest) (*pluginv1.ResolveImageURLResponse, error)
	ResolveImageURLs(ctx context.Context, req *pluginv1.ResolveImageURLsRequest) (*pluginv1.ResolveImageURLsResponse, error)
}

// PluginMetadataClientFactory creates a PluginMetadataClient for a given plugin installation.
type PluginMetadataClientFactory func(ctx context.Context, installationID int, capabilityID string) (PluginMetadataClient, error)

// pluginClientSource wraps a PluginMetadataClientFactory to satisfy PluginImageResolverSource.
type pluginClientSource struct {
	installationID int
	capabilityID   string
	clientFactory  PluginMetadataClientFactory
}

// NewPluginClientSource creates a PluginImageResolverSource from a plugin metadata client factory.
func NewPluginClientSource(installationID int, capabilityID string, factory PluginMetadataClientFactory) PluginImageResolverSource {
	return &pluginClientSource{
		installationID: installationID,
		capabilityID:   capabilityID,
		clientFactory:  factory,
	}
}

func (s *pluginClientSource) ResolveImageURL(ctx context.Context, path string, variant string) (string, error) {
	resolved, err := s.ResolveImageURLWithExpiry(ctx, path, variant)
	if err != nil {
		return "", err
	}
	return resolved.URL, nil
}

func (s *pluginClientSource) ResolveImageURLWithExpiry(ctx context.Context, path string, variant string) (catalog.ResolvedImageURL, error) {
	client, err := s.clientFactory(ctx, s.installationID, s.capabilityID)
	if err != nil {
		return catalog.ResolvedImageURL{}, err
	}

	resp, err := client.ResolveImageURL(ctx, &pluginv1.ResolveImageURLRequest{Path: path, Variant: variant})
	if err != nil {
		return catalog.ResolvedImageURL{}, err
	}
	return catalog.ResolvedImageURL{URL: resp.GetUrl()}, nil
}

func (s *pluginClientSource) ResolveImageURLs(ctx context.Context, paths []string, variant string) (map[string]string, error) {
	resolvedWithExpiry, err := s.ResolveImageURLsWithExpiry(ctx, paths, variant)
	if err != nil {
		return nil, err
	}
	resolved := make(map[string]string, len(resolvedWithExpiry))
	for path, value := range resolvedWithExpiry {
		resolved[path] = value.URL
	}
	return resolved, nil
}

func (s *pluginClientSource) ResolveImageURLsWithExpiry(ctx context.Context, paths []string, variant string) (map[string]catalog.ResolvedImageURL, error) {
	client, err := s.clientFactory(ctx, s.installationID, s.capabilityID)
	if err != nil {
		return nil, err
	}

	resp, err := client.ResolveImageURLs(ctx, &pluginv1.ResolveImageURLsRequest{Paths: paths, Variant: variant})
	if err != nil {
		return nil, err
	}

	resolved := make(map[string]catalog.ResolvedImageURL, len(paths))
	for path, url := range resp.GetUrls() {
		resolved[path] = catalog.ResolvedImageURL{URL: url}
	}
	return resolved, nil
}

// parsePluginPrefix extracts the plugin ID and bare path from a prefixed path.
// Input: "metadb://images/abc/original.jpg"
// Returns: ("metadb", "images/abc/original.jpg")
func parsePluginPrefix(path string) (pluginID, barePath string) {
	idx := strings.Index(path, "://")
	if idx <= 0 {
		return "", ""
	}
	return path[:idx], path[idx+3:]
}
