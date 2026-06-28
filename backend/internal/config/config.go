// Package config loads runtime configuration from environment variables.
//
// Values map to backend/.env.example. A .env file is loaded when present
// (development convenience); in production the process environment is used.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime settings for the Inventra backend.
type Config struct {
	Env        string
	ServerPort string

	// PostgreSQL — authoritative data store.
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Redis — cache, sessions, rate limiting, TTL tokens (complementary, not source of truth).
	RedisAddr     string
	RedisPassword string
	RedisDB       int

	// MinIO — S3-compatible object storage for asset attachments.
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	MinIOUseSSL        bool
	AttachmentMaxBytes int64

	// Auth.
	JWTSecret     string
	JWTAccessTTL  time.Duration
	JWTRefreshTTL time.Duration

	// Google OAuth2.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	GoogleIssuer       string

	FrontendURL string

	// Logging (ADR-0002).
	LogLevel  string
	LogFormat string

	// Rate limiting (ADR-0004).
	RateLimitEnabled       bool
	RateLimitTimeoutMS     int
	RateLimitGlobalPerMin  int
	RateLimitLoginPerMin   int
	RateLimitLoginIPPerMin int
	RateLimitRefreshPerMin int

	// TrustedProxies lists the CIDRs/IPs whose X-Forwarded-For header Gin should
	// honour when resolving c.ClientIP() for rate limiting. nil = trust none
	// (use direct RemoteAddr, which cannot be spoofed). Set to your load-balancer
	// CIDR(s) in production via TRUSTED_PROXIES (comma-separated).
	TrustedProxies []string

	// Label printing.
	LabelLogoPath string
}

// Load reads configuration from the environment, applying sensible development
// defaults. It loads a .env file first if one exists.
func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		Env:        getEnv("ENV", "development"),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5433"),
		DBUser:     getEnv("DB_USER", "inventra"),
		DBPassword: getEnv("DB_PASSWORD", "secret"),
		DBName:     getEnv("DB_NAME", "inventra_dev"),
		DBSSLMode:  getEnv("DB_SSLMODE", "disable"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		MinIOEndpoint:      getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:     getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:     getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		MinIOBucket:        getEnv("MINIO_BUCKET", "inventra"),
		MinIOUseSSL:        getEnvBool("MINIO_USE_SSL", false),
		AttachmentMaxBytes: int64(getEnvInt("ATTACHMENT_MAX_BYTES", 5*1024*1024)),

		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		JWTAccessTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 168*time.Hour),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),
		GoogleIssuer:       getEnv("GOOGLE_ISSUER", "https://accounts.google.com"),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),

		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", ""),

		RateLimitEnabled:       getEnvBool("RATELIMIT_ENABLED", true),
		RateLimitTimeoutMS:     getEnvInt("RATELIMIT_TIMEOUT_MS", 50),
		RateLimitGlobalPerMin:  getEnvInt("RATELIMIT_GLOBAL_PER_MIN", 120),
		RateLimitLoginPerMin:   getEnvInt("RATELIMIT_LOGIN_PER_MIN", 5),
		RateLimitLoginIPPerMin: getEnvInt("RATELIMIT_LOGIN_IP_PER_MIN", 20),
		RateLimitRefreshPerMin: getEnvInt("RATELIMIT_REFRESH_PER_MIN", 30),

		TrustedProxies: splitCSV(getEnv("TRUSTED_PROXIES", "")),

		LabelLogoPath: getEnv("LABEL_LOGO_PATH", "assets/btn-logo.png"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// splitCSV parses a comma-separated value into a trimmed, non-empty slice (nil if empty).
func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}
