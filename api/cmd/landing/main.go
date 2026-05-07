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

	// Admin handle for cross-tenant slug lookup.
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	// App handle for INSERTs into RLS-subject tables (registrations).
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false)
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()

	orgRepo := repo.NewOrganizationRepo(adminDB)
	regRepo := repo.NewRegistrationRepo(appDB)

	handler, err := landing.New(orgRepo, regRepo, logger)
	if err != nil {
		slog.Error("init landing handler", "error", err)
		os.Exit(1)
	}

	// Middleware chain: request_id → logger → RLSContext → routes.
	// Tenant ctx is set INSIDE the handler (slug from path, not Host), so the
	// HTTP-level tenant resolver does not apply here. RLSContext picks up the
	// tenant from ctx (set by the handler) and runs SET LOCAL for the duration
	// of the request. Routes that don't carry a tenant (like /static/*) flow
	// through unchanged.
	chain := middleware.RequestID(
		middleware.Logger(logger)(
			middleware.RLSContext(middleware.NewBunRunner(appDB))(
				handler.Routes(),
			),
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
