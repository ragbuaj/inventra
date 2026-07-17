package config

import (
	"testing"
	"time"
)

func TestLoadNotificationDefaults(t *testing.T) {
	t.Setenv("NOTIFICATION_WORKER_ENABLED", "")
	t.Setenv("NOTIFICATION_RELAY_POLL", "")
	t.Setenv("NOTIFICATION_STREAM_MAXLEN", "")
	cfg := Load()
	if !cfg.NotificationWorkerEnabled {
		t.Fatal("NotificationWorkerEnabled default should be true")
	}
	if cfg.NotificationRelayPoll != 2*time.Second {
		t.Fatalf("NotificationRelayPoll default: %v", cfg.NotificationRelayPoll)
	}
	if cfg.NotificationStreamMaxLen != 10000 {
		t.Fatalf("NotificationStreamMaxLen default: %d", cfg.NotificationStreamMaxLen)
	}
}

func TestLoadNotificationFromEnv(t *testing.T) {
	t.Setenv("NOTIFICATION_WORKER_ENABLED", "false")
	t.Setenv("NOTIFICATION_RELAY_POLL", "500ms")
	t.Setenv("NOTIFICATION_STREAM_MAXLEN", "250")
	cfg := Load()
	if cfg.NotificationWorkerEnabled {
		t.Fatal("NOTIFICATION_WORKER_ENABLED=false not applied")
	}
	if cfg.NotificationRelayPoll != 500*time.Millisecond {
		t.Fatalf("NOTIFICATION_RELAY_POLL: %v", cfg.NotificationRelayPoll)
	}
	if cfg.NotificationStreamMaxLen != 250 {
		t.Fatalf("NOTIFICATION_STREAM_MAXLEN: %d", cfg.NotificationStreamMaxLen)
	}
}

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

func TestLoadTrustedProxies(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "10.0.0.0/8, 192.168.1.1 ,")
	cfg := Load()
	if len(cfg.TrustedProxies) != 2 || cfg.TrustedProxies[0] != "10.0.0.0/8" || cfg.TrustedProxies[1] != "192.168.1.1" {
		t.Fatalf("parsed: %#v", cfg.TrustedProxies)
	}
}

func TestLoadTrustedProxiesEmpty(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	cfg := Load()
	if cfg.TrustedProxies != nil {
		t.Fatalf("empty TRUSTED_PROXIES should yield nil, got %#v", cfg.TrustedProxies)
	}
}

func TestLoadGoogleIssuerDefault(t *testing.T) {
	t.Setenv("GOOGLE_ISSUER", "")
	if got := Load().GoogleIssuer; got != "https://accounts.google.com" {
		t.Fatalf("GoogleIssuer default: %q", got)
	}
}
