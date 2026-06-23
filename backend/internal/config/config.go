// Package config loads runtime configuration from environment variables.
//
// Values map to backend/.env.example. A .env file is loaded when present
// (development convenience); in production the process environment is used.
package config

import (
	"os"
	"strconv"

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
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
	MinIOUseSSL    bool

	// Auth.
	JWTSecret string

	// Google OAuth2.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	FrontendURL string
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

		MinIOEndpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),
		MinIOBucket:    getEnv("MINIO_BUCKET", "inventra"),
		MinIOUseSSL:    getEnvBool("MINIO_USE_SSL", false),

		JWTSecret: getEnv("JWT_SECRET", "change-me-in-production"),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/auth/google/callback"),

		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
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
