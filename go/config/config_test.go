package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://postgres:postgres@localhost:5432/delta" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.AccessTokenTTL != 15*time.Minute {
		t.Errorf("AccessTokenTTL = %v", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("RefreshTokenTTL = %v", cfg.RefreshTokenTTL)
	}
	if !cfg.AutoMigrate || !cfg.TrustProxyHeaders {
		t.Errorf("AutoMigrate=%v TrustProxyHeaders=%v", cfg.AutoMigrate, cfg.TrustProxyHeaders)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("JWTSecret = %q", cfg.JWTSecret)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("JWT_SECRET", "s")
	t.Setenv("PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://u:p@h/db")
	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("JWT_ACCESS_TOKEN_TTL", "60000")
	t.Setenv("TRUST_PROXY_HEADERS", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != 9090 || cfg.DatabaseURL != "postgres://u:p@h/db" || cfg.AutoMigrate ||
		cfg.AccessTokenTTL != time.Minute || cfg.TrustProxyHeaders {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error when JWT_SECRET empty")
	}
}
