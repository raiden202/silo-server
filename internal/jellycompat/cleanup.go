package jellycompat

import (
	"context"
	"log/slog"
	"time"
)

// StartSessionCleanup runs a background goroutine that periodically removes
// expired compat sessions from the database. It stops when ctx is cancelled.
func StartSessionCleanup(ctx context.Context, repo *SessionRepository, interval time.Duration) {
	if repo == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				deleted, err := repo.DeleteExpired(ctx, time.Now())
				if err != nil {
					slog.Warn("jellycompat session cleanup failed", "error", err)
					continue
				}
				if deleted > 0 {
					slog.Debug("jellycompat session cleanup", "deleted", deleted)
				}
			}
		}
	}()
}
