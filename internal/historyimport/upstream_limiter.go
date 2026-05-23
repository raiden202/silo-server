package historyimport

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/time/rate"
)

const (
	historyImportUpstreamRequestsPerSecond = 2
	historyImportUpstreamBurst             = 1
)

var sharedHistoryImportUpstreamLimiter = newUpstreamRateLimiter(
	rate.Limit(historyImportUpstreamRequestsPerSecond),
	historyImportUpstreamBurst,
)

type upstreamRateLimiter struct {
	mu       sync.Mutex
	limit    rate.Limit
	burst    int
	limiters map[string]*rate.Limiter
}

func newUpstreamRateLimiter(limit rate.Limit, burst int) *upstreamRateLimiter {
	if burst <= 0 {
		burst = 1
	}
	return &upstreamRateLimiter{
		limit:    limit,
		burst:    burst,
		limiters: make(map[string]*rate.Limiter),
	}
}

func (l *upstreamRateLimiter) Wait(ctx context.Context, reqURL *url.URL) error {
	if l == nil || reqURL == nil {
		return nil
	}
	key := limiterKey(reqURL)
	if key == "" {
		return nil
	}

	return l.limiterFor(key).Wait(ctx)
}

func (l *upstreamRateLimiter) limiterFor(key string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if limiter, ok := l.limiters[key]; ok {
		return limiter
	}
	limiter := rate.NewLimiter(l.limit, l.burst)
	l.limiters[key] = limiter
	return limiter
}

func limiterKey(reqURL *url.URL) string {
	if reqURL == nil {
		return ""
	}
	return strings.ToLower(reqURL.Host)
}
