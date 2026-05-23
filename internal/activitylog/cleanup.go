package activitylog

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	keyRetentionDays    = "activitylog.retention_days"
	defaultRetentionStr = "90"
	defaultRetention    = 90
	cleanupBatchSize    = 10000
)

// SettingsStore is satisfied by *catalog.ServerSettingsRepo.
type SettingsStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type PartitionManager interface {
	EnsureFuturePartitions(ctx context.Context) error
	DropExpiredPartitions(ctx context.Context, cutoff time.Time) ([]string, error)
	DeleteExpiredRowsFromDefault(ctx context.Context, cutoff time.Time) (int64, error)
}

// SeedDefaults writes default activity log settings if not already set.
func SeedDefaults(ctx context.Context, store SettingsStore) error {
	existing, err := store.Get(ctx, keyRetentionDays)
	if err != nil {
		return fmt.Errorf("seed activitylog defaults: %w", err)
	}
	if existing != "" {
		return nil
	}
	return store.Set(ctx, keyRetentionDays, defaultRetentionStr)
}

// RunCleanup starts a background goroutine that runs batched deletes daily.
// Blocks until ctx is cancelled.
func RunCleanup(ctx context.Context, pool *pgxpool.Pool, store SettingsStore, pm PartitionManager) {
	// Run once at startup, then every 24 hours
	CleanupOnce(ctx, pool, store, pm)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			CleanupOnce(ctx, pool, store, pm)
		}
	}
}

// CleanupOnce runs a single activity log retention pass.
func CleanupOnce(ctx context.Context, pool *pgxpool.Pool, store SettingsStore, pm PartitionManager) int64 {
	days := defaultRetention
	if raw, err := store.Get(ctx, keyRetentionDays); err == nil && raw != "" {
		if d := parseInt(raw); d > 0 {
			days = d
		}
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	if pm != nil {
		if err := pm.EnsureFuturePartitions(ctx); err != nil {
			slog.Warn("activitylog ensure future partitions error", "error", err)
		}

		partitionCleanupFailed := false
		totalDeleted := int64(0)
		if dropped, err := pm.DropExpiredPartitions(ctx, cutoff); err != nil {
			slog.Warn("activitylog partition cleanup error", "error", err)
			partitionCleanupFailed = true
		} else if len(dropped) > 0 {
			slog.Info("activitylog dropped expired partitions", "partitions", dropped)
		}

		if deleted, err := pm.DeleteExpiredRowsFromDefault(ctx, cutoff); err != nil {
			slog.Warn("activitylog default partition cleanup error", "error", err)
			partitionCleanupFailed = true
		} else if deleted > 0 {
			totalDeleted += deleted
			slog.Info("activitylog default partition cleanup completed", "deleted", deleted, "retention_days", days)
		}

		if !partitionCleanupFailed {
			return totalDeleted
		}
		slog.Warn("activitylog partition cleanup degraded, falling back to row deletes", "retention_days", days)
	}

	total := deleteExpiredRowsBefore(ctx, pool, cutoff)
	if total > 0 {
		slog.Info("activitylog cleanup completed", "deleted", total, "retention_days", days)
	}
	return total
}

func deleteExpiredRowsBefore(ctx context.Context, pool *pgxpool.Pool, cutoff time.Time) int64 {
	total := int64(0)
	for {
		result, err := pool.Exec(ctx, `
			DELETE FROM activity_log
			WHERE id IN (
				SELECT id FROM activity_log
				WHERE timestamp < $1
				LIMIT $2
			)
			`, cutoff, cleanupBatchSize)
		if err != nil {
			slog.Warn("activitylog cleanup error", "error", err)
			return total
		}
		deleted := result.RowsAffected()
		total += deleted
		if deleted < int64(cleanupBatchSize) {
			break
		}
	}
	return total
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}
