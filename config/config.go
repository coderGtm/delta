// Package config loads the service configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the service configuration loaded from environment variables.
type Config struct {
	Port                       int           // HTTP listen port.
	DatabaseURL                string        // PostgreSQL connection URL.
	AutoMigrate                bool          // Whether to apply migrations automatically at startup.
	JWTSecret                  string        // Secret used to sign access and refresh tokens.
	AccessTokenTTL             time.Duration // Access token lifetime.
	RefreshTokenTTL            time.Duration // Refresh token lifetime.
	RefreshCleanupInterval     time.Duration // How often expired tokens are cleaned up.
	RefreshRevokedRetention    time.Duration // How long revoked tokens are kept before permanent deletion.
	FirebaseServiceAccountPath string        // Path to the Firebase service account JSON file.
	PrometheusBearerToken      string        // Bearer token required to access the metrics endpoint.
	TrustProxyHeaders          bool          // Whether X-Forwarded-For and X-Real-IP headers are honored.
	LogLevel                   string        // Log level, e.g. "info" or "debug".
	LogFormat                  string        // Log output format, "text" or "json".
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvDurationMillis(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return def
}

// Load reads the service configuration from environment variables, applying
// defaults where a variable is unset. It returns an error when JWT_SECRET is
// not set.
func Load() (Config, error) {
	cfg := Config{
		Port:                       getEnvInt("PORT", 8080),
		DatabaseURL:                getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/delta"),
		AutoMigrate:                getEnvBool("AUTO_MIGRATE", true),
		JWTSecret:                  os.Getenv("JWT_SECRET"),
		AccessTokenTTL:             getEnvDurationMillis("JWT_ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:            getEnvDurationMillis("JWT_REFRESH_TOKEN_TTL", 30*24*time.Hour),
		RefreshCleanupInterval:     getEnvDurationMillis("JWT_REFRESH_CLEANUP_INTERVAL", 24*time.Hour),
		RefreshRevokedRetention:    getEnvDurationMillis("JWT_REFRESH_REVOKED_RETENTION", 7*24*time.Hour),
		FirebaseServiceAccountPath: getEnv("FIREBASE_SERVICE_ACCOUNT_PATH", "firebase/service-account.json"),
		PrometheusBearerToken:      os.Getenv("PROMETHEUS_BEARER_TOKEN"),
		TrustProxyHeaders:          getEnvBool("TRUST_PROXY_HEADERS", true),
		LogLevel:                   getEnv("LOG_LEVEL", "info"),
		LogFormat:                  getEnv("LOG_FORMAT", "text"),
	}
	if cfg.JWTSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET must be set")
	}
	return cfg, nil
}
