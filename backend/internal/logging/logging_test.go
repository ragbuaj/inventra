package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/ragbuaj/inventra/internal/config"
)

func bufJSON(level slog.Level) (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	return slog.New(h), &buf
}

func TestNewHonorsLevel(t *testing.T) {
	l := New(&config.Config{Env: "production", LogLevel: "warn"})
	if l.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("info must be disabled at warn level")
	}
	if !l.Enabled(context.Background(), slog.LevelWarn) {
		t.Fatal("warn must be enabled")
	}
}

func TestUseJSONResolution(t *testing.T) {
	if useJSON(&config.Config{Env: "development"}) {
		t.Fatal("dev default should be text")
	}
	if !useJSON(&config.Config{Env: "production"}) {
		t.Fatal("prod default should be json")
	}
	if !useJSON(&config.Config{Env: "development", LogFormat: "json"}) {
		t.Fatal("LOG_FORMAT=json overrides dev")
	}
	if useJSON(&config.Config{Env: "production", LogFormat: "text"}) {
		t.Fatal("LOG_FORMAT=text overrides prod")
	}
}

func TestRedactionTopLevel(t *testing.T) {
	l, buf := bufJSON(slog.LevelInfo)
	l.Info("login", "email", "a@b.com", "password", "hunter2", "google_id", "xyz")
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["password"] != "[REDACTED]" || m["google_id"] != "[REDACTED]" {
		t.Fatalf("sensitive keys not redacted: %v", m)
	}
	if m["email"] != "a@b.com" {
		t.Fatalf("non-sensitive key altered: %v", m["email"])
	}
}

func TestRedactionInGroup(t *testing.T) {
	l, buf := bufJSON(slog.LevelInfo)
	l.Info("req", slog.Group("auth", "token", "secret-value"))
	s := buf.String()
	if strings.Contains(s, "secret-value") || !strings.Contains(s, "[REDACTED]") {
		t.Fatalf("token inside group not redacted: %s", s)
	}
}

func TestContextRoundTrip(t *testing.T) {
	if FromContext(context.Background()) == nil {
		t.Fatal("must fall back to a non-nil default")
	}
	custom, _ := bufJSON(slog.LevelInfo)
	ctx := WithLogger(context.Background(), custom)
	if FromContext(ctx) != custom {
		t.Fatal("must return the stored logger")
	}
}
