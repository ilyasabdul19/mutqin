// api/cmd/landing/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler/landing"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Admin handle for cross-tenant slug lookup AND for running migrations on
	// startup (Lock() coordinates with the api binary if both run concurrently).
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	// App handle for INSERTs into RLS-subject tables (registrations). Connects
	// AFTER migrations have run so the mutqin_app role definitely exists.
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false)
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()

	orgRepo := repo.NewOrganizationRepo(adminDB)
	announcementRepo := repo.NewAnnouncementRepo(adminDB)

	handler, err := landing.New(orgRepo, announcementRepo, appDB, logger)
	if err != nil {
		slog.Error("init landing handler", "error", err)
		os.Exit(1)
	}

	// Middleware chain: request_id → logger → routes.
	// Tenant ctx + RLS context are set INSIDE the submit handler — it wraps
	// the INSERT in a transaction with SET LOCAL app.current_tenant. The
	// HTTP-level tenant resolver / RLSContext middlewares are not used here
	// because the slug comes from the URL path, not the Host header, and the
	// landing routes are public (no auth) so per-request DB transactions are
	// only needed on writes.
	chain := middleware.RequestID(
		middleware.Logger(logger)(
			handler.Routes(),
		),
	)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           chain,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("landing server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("landing server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("landing server shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("landing server shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("landing server stopped")
}
