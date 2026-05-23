package metadata

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Silo-Server/silo-server/internal/catalog"
)

type fakeExpiringImageSource struct {
	expiresAt *time.Time
	delay     time.Duration
	calls     atomic.Int32
}

func (s *fakeExpiringImageSource) ResolveImageURL(ctx context.Context, path string, variant string) (string, error) {
	resolved, err := s.ResolveImageURLWithExpiry(ctx, path, variant)
	return resolved.URL, err
}

func (s *fakeExpiringImageSource) ResolveImageURLWithExpiry(ctx context.Context, path string, variant string) (catalog.ResolvedImageURL, error) {
	resolved, err := s.ResolveImageURLsWithExpiry(ctx, []string{path}, variant)
	if err != nil {
		return catalog.ResolvedImageURL{}, err
	}
	return resolved[path], nil
}

func (s *fakeExpiringImageSource) ResolveImageURLs(ctx context.Context, paths []string, variant string) (map[string]string, error) {
	resolved, err := s.ResolveImageURLsWithExpiry(ctx, paths, variant)
	if err != nil {
		return nil, err
	}
	urls := make(map[string]string, len(resolved))
	for path, value := range resolved {
		urls[path] = value.URL
	}
	return urls, nil
}

func (s *fakeExpiringImageSource) ResolveImageURLsWithExpiry(ctx context.Context, paths []string, variant string) (map[string]catalog.ResolvedImageURL, error) {
	s.calls.Add(1)
	if s.delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(s.delay):
		}
	}
	resolved := make(map[string]catalog.ResolvedImageURL, len(paths))
	for _, path := range paths {
		resolved[path] = catalog.ResolvedImageURL{
			URL:       "plugin:" + variant + ":" + path,
			ExpiresAt: s.expiresAt,
		}
	}
	return resolved, nil
}

func TestPluginImageResolverCachesOnlyKnownUsableExpiries(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	source := &fakeExpiringImageSource{expiresAt: &expiresAt}
	resolver := NewPluginImageResolver()
	defer resolver.Close()
	resolver.RegisterSource("plug", source)

	for range 2 {
		got := resolver.ResolveImageURLWithExpiry(context.Background(), "plug://poster.jpg", "featured")
		if got.URL != "plugin:featured:poster.jpg" {
			t.Fatalf("resolved URL = %q", got.URL)
		}
	}
	if calls := source.calls.Load(); calls != 1 {
		t.Fatalf("plugin calls with usable expiry = %d, want 1", calls)
	}

	noExpirySource := &fakeExpiringImageSource{}
	noExpiryResolver := NewPluginImageResolver()
	defer noExpiryResolver.Close()
	noExpiryResolver.RegisterSource("plug", noExpirySource)
	for range 2 {
		_ = noExpiryResolver.ResolveImageURLWithExpiry(context.Background(), "plug://poster.jpg", "featured")
	}
	if calls := noExpirySource.calls.Load(); calls != 2 {
		t.Fatalf("plugin calls without expiry = %d, want 2", calls)
	}

	nearExpiry := time.Now().Add(time.Minute)
	nearExpirySource := &fakeExpiringImageSource{expiresAt: &nearExpiry}
	nearExpiryResolver := NewPluginImageResolver()
	defer nearExpiryResolver.Close()
	nearExpiryResolver.RegisterSource("plug", nearExpirySource)
	for range 2 {
		_ = nearExpiryResolver.ResolveImageURLWithExpiry(context.Background(), "plug://poster.jpg", "featured")
	}
	if calls := nearExpirySource.calls.Load(); calls != 2 {
		t.Fatalf("plugin calls with near expiry = %d, want 2", calls)
	}
}

func TestPluginImageResolverCoalescesConcurrentBatchMisses(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	source := &fakeExpiringImageSource{expiresAt: &expiresAt, delay: 50 * time.Millisecond}
	resolver := NewPluginImageResolver()
	defer resolver.Close()
	resolver.RegisterSource("plug", source)

	const workers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := range workers {
		go func(i int) {
			defer wg.Done()
			<-start
			resolved := resolver.ResolveImageURLsWithExpiry(context.Background(), []string{"plug://a.jpg", "plug://b.jpg"}, "featured")
			if got := resolved["plug://a.jpg"].URL; got != "plugin:featured:a.jpg" {
				t.Errorf("worker %d resolved a.jpg = %q", i, got)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if calls := source.calls.Load(); calls != 1 {
		t.Fatalf("plugin calls = %d, want 1", calls)
	}
}

type fakeS3ImagePresigner struct {
	calls int
	ttl   time.Duration
}

func (p *fakeS3ImagePresigner) PresignGetURL(_ context.Context, _ string, key string, expiry time.Duration) (string, error) {
	p.calls++
	p.ttl = expiry
	return fmt.Sprintf("s3:%s:%d", key, p.calls), nil
}

func (p *fakeS3ImagePresigner) Bucket() string {
	return "metadata"
}

func TestPluginImageResolverS3URLsCarryConfiguredExpiry(t *testing.T) {
	presigner := &fakeS3ImagePresigner{}
	resolver := NewPluginImageResolver()
	defer resolver.Close()
	resolver.SetS3Presigner(presigner, 10*time.Minute)

	before := time.Now()
	first := resolver.ResolveImageURLWithExpiry(context.Background(), "poster.jpg", "featured")
	second := resolver.ResolveImageURLWithExpiry(context.Background(), "poster.jpg", "featured")

	if first.URL == "" || second.URL != first.URL {
		t.Fatalf("cached S3 URLs = first %q second %q", first.URL, second.URL)
	}
	if presigner.calls != 1 {
		t.Fatalf("s3 presign calls = %d, want 1", presigner.calls)
	}
	if presigner.ttl != 10*time.Minute {
		t.Fatalf("s3 presign ttl = %s, want 10m", presigner.ttl)
	}
	if first.ExpiresAt == nil {
		t.Fatal("S3 resolved URL missing expiry")
	}
	if first.ExpiresAt.Before(before.Add(9*time.Minute)) || first.ExpiresAt.After(before.Add(11*time.Minute)) {
		t.Fatalf("S3 expiry = %s, want about 10m from now", first.ExpiresAt.Sub(before))
	}
}
