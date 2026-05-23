package opslog

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

const redisKey = "ops_log:buffer"

type RedisWriter struct {
	client *redis.Client
}

func NewRedisWriter(client *redis.Client) *RedisWriter {
	return &RedisWriter{client: client}
}

func (w *RedisWriter) Write(entry Entry) {
	data, err := json.Marshal(entry)
	if err != nil {
		slog.Warn("opslog: failed to marshal entry", "error", err)
		return
	}
	if err := w.client.RPush(context.Background(), redisKey, data).Err(); err != nil {
		slog.Warn("opslog: failed to push to Redis", "error", err)
	}
}

func (w *RedisWriter) Close() error {
	return nil
}
