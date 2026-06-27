package config

import "testing"

func TestLoadLoggingDefaults(t *testing.T) {
	cfg := Load()
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel default: %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "" {
		t.Fatalf("LogFormat default should be empty (auto): %q", cfg.LogFormat)
	}
}

func TestLoadLoggingFromEnv(t *testing.T) {
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")
	cfg := Load()
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("env not applied: level=%q format=%q", cfg.LogLevel, cfg.LogFormat)
	}
}
