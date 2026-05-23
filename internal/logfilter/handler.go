package logfilter

import (
	"context"
	"log/slog"
	"strings"
)

// Handler wraps an slog.Handler and drops records whose message starts
// with any of the configured quiet prefixes (e.g. "metadata:").
type Handler struct {
	inner    slog.Handler
	prefixes []string
}

// New returns a Handler that delegates to inner but silences messages
// matching any prefix in quietCSV (comma-separated, e.g. "metadata,scanner").
// A colon+space is appended automatically: "metadata" silences "metadata: ...".
// If quietCSV is empty, the inner handler is returned directly.
func New(inner slog.Handler, quietCSV string) slog.Handler {
	trimmed := strings.TrimSpace(quietCSV)
	if trimmed == "" {
		return inner
	}
	parts := strings.Split(trimmed, ",")
	prefixes := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			prefixes = append(prefixes, p+":")
		}
	}
	if len(prefixes) == 0 {
		return inner
	}
	return &Handler{inner: inner, prefixes: prefixes}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	msg := r.Message
	for _, prefix := range h.prefixes {
		if strings.HasPrefix(msg, prefix) {
			return nil
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{inner: h.inner.WithAttrs(attrs), prefixes: h.prefixes}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{inner: h.inner.WithGroup(name), prefixes: h.prefixes}
}
