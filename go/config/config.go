package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                       int
	DatabaseURL                string
	AutoMigrate                bool
	JWTSecret                  string
	AccessTokenTTL             time.Duration
	RefreshTokenTTL            time.Duration
	RefreshCleanupInterval     time.Duration
	RefreshRevokedRetention    time.Duration
	FirebaseServiceAccountPath string
	PrometheusBearerToken      string
	TrustProxyHeaders          bool
	LogLevel                   string
	LogFormat                  string
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
