package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/coderGtm/delta/go/audit"
	"github.com/coderGtm/delta/go/auth"
	"github.com/coderGtm/delta/go/config"
	"github.com/coderGtm/delta/go/db"
	"github.com/coderGtm/delta/go/httpapi"
	"github.com/coderGtm/delta/go/metrics"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	logger := newLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
			logger.Error("migrate", "err", err)
			os.Exit(1)
		}
	}

	registry := metrics.NewRegistry()

	store := db.NewStore(pool)

	fb, err := auth.NewFirebaseClient(ctx, cfg.FirebaseServiceAccountPath)
	if err != nil {
		logger.Warn("firebase client unavailable; login will reject tokens", "err", err, "path", cfg.FirebaseServiceAccountPath)
	}
	jwtSvc := auth.NewJWTService(cfg.JWTSecret, cfg.AccessTokenTTL)
	refreshSvc := auth.NewRefreshTokenService(store, cfg.RefreshTokenTTL, cfg.RefreshRevokedRetention)
	recorder := audit.NewRecorder(store)
	authSvc := auth.NewService(store, fb, jwtSvc, refreshSvc, recorder, registry)
	authHandlers := &auth.Handlers{Svc: authSvc, TrustProxy: cfg.TrustProxyHeaders}

	apiMux := http.NewServeMux()
	apiMux.Handle("POST /api/v1/auth/login", http.HandlerFunc(authHandlers.Login))
	apiMux.Handle("POST /api/v1/auth/refresh", http.HandlerFunc(authHandlers.Refresh))
	apiMux.Handle("POST /api/v1/auth/logout", http.HandlerFunc(authHandlers.Logout))
	apiMux.Handle("POST /api/v1/auth/logout-all", auth.Require(http.HandlerFunc(authHandlers.LogoutAll)))

	go refreshSvc.RunCleanupTicker(ctx, cfg.RefreshCleanupInterval)

	ready := func(ctx context.Context) error {
		return pool.Ping(ctx)
	}

	handler := httpapi.NewRouter(logger, cfg, ready, registry.Handler(), auth.AttachUser(jwtSvc, store), apiMux)

	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Info("server started", "addr", srv.Addr)

	select {
	case err := <-errCh:
		logger.Error("server error", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown", "err", err)
		}
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	opts := &slog.HandlerOptions{Level: level}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
