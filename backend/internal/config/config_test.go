package config

import "testing"

func TestLoadLoggingDefaults(t *testing.T) {
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")
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

func TestLoadRateLimitDefaults(t *testing.T) {
	t.Setenv("RATELIMIT_ENABLED", "")
	t.Setenv("RATELIMIT_TIMEOUT_MS", "")
	t.Setenv("RATELIMIT_GLOBAL_PER_MIN", "")
	t.Setenv("RATELIMIT_LOGIN_PER_MIN", "")
	t.Setenv("RATELIMIT_LOGIN_IP_PER_MIN", "")
	t.Setenv("RATELIMIT_REFRESH_PER_MIN", "")
	cfg := Load()
	if !cfg.RateLimitEnabled {
		t.Fatal("RateLimitEnabled default should be true")
	}
	if cfg.RateLimitGlobalPerMin != 120 || cfg.RateLimitLoginPerMin != 5 ||
		cfg.RateLimitLoginIPPerMin != 20 || cfg.RateLimitRefreshPerMin != 30 || cfg.RateLimitTimeoutMS != 50 {
		t.Fatalf("unexpected rate-limit defaults: %+v", cfg)
	}
}

func TestLoadRateLimitFromEnv(t *testing.T) {
	t.Setenv("RATELIMIT_ENABLED", "false")
	t.Setenv("RATELIMIT_LOGIN_PER_MIN", "9")
	cfg := Load()
	if cfg.RateLimitEnabled {
		t.Fatal("RATELIMIT_ENABLED=false not applied")
	}
	if cfg.RateLimitLoginPerMin != 9 {
		t.Fatalf("RATELIMIT_LOGIN_PER_MIN: %d", cfg.RateLimitLoginPerMin)
	}
}
