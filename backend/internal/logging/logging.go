// Package logging builds the application's structured slog logger and provides
// request-scoped logger propagation via context. Sensitive fields are redacted
// at the handler level (see ADR-0002).
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/ragbuaj/inventra/internal/config"
)

const redacted = "[REDACTED]"

// sensitiveKeys are never written in clear text.
var sensitiveKeys = map[string]struct{}{
	"password": {}, "password_hash": {}, "token": {}, "access_token": {},
	"refresh_token": {}, "secret": {}, "authorization": {}, "google_id": {}, "api_key": {},
}

type ctxKey struct{}

// New builds the app logger from config. It does NOT call slog.SetDefault —
// the caller (main) does that explicitly.
func New(cfg *config.Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel), ReplaceAttr: redactAttr}
	var h slog.Handler
	if useJSON(cfg) {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// useJSON resolves the output format: explicit LOG_FORMAT wins, else json
// unless the environment is development.
func useJSON(cfg *config.Config) bool {
	switch strings.ToLower(cfg.LogFormat) {
	case "json":
		return true
	case "text":
		return false
	default:
		return cfg.Env != "development"
	}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// redactAttr masks sensitive values regardless of type or nesting depth.
func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, redacted)
	}
	return a
}

// WithLogger stores a request-scoped logger on the context.
func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the context's logger, or slog.Default() if absent.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
