package activitylog

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const redisKey = "activity_log:buffer"

// RedisWriter pushes log entries to a Redis list for async consumption.
type RedisWriter struct {
	client *redis.Client
}

// NewRedisWriter creates a Writer backed by Redis RPUSH.
func NewRedisWriter(client *redis.Client) *RedisWriter {
	return &RedisWriter{client: client}
}

func (w *RedisWriter) Write(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("activitylog: failed to marshal entry", "error", err)
		return
	}
	if err := w.client.RPush(context.Background(), redisKey, data).Err(); err != nil {
		slog.Warn("activitylog: failed to push to Redis", "error", err)
	}
}

func (w *RedisWriter) Close() error {
	// Redis client lifecycle managed externally
	return nil
}
