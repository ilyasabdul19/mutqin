// api/cmd/server/main.go
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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/repo"
)

type orgLookupAdapter struct{ r *repo.OrganizationRepo }

func (a orgLookupAdapter) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	org, err := a.r.GetBySlugAdmin(ctx, slug)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	migrateUp := flag.Bool("migrate-up", false, "apply pending migrations and exit")
	migrateDown := flag.Bool("migrate-down", false, "roll back the most recent migration group and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Admin/superuser handle — used for migrations and cross-tenant lookups
	// (e.g. resolving a slug to an org UUID before the tenant ctx is set).
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	switch {
	case *migrateUp:
		if err := migrate.Up(ctx, adminDB); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
		return
	case *migrateDown:
		if err := migrate.Down(ctx, adminDB); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	// App handle — non-superuser, RLS-subject. Per-model hooks via
	// model.TenantScoped inject organization_id filters when ctx has a tenant;
	// Postgres RLS enforces at the storage layer regardless.
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false)
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()
	slog.Info("connected to database",
		"admin_dsn_redacted", redactDSN(cfg.DatabaseURL),
		"app_dsn_redacted", redactDSN(cfg.AppDatabaseURL))

	orgLookup := orgLookupAdapter{r: repo.NewOrganizationRepo(adminDB)}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS)
	r.Use(middleware.Tenant(orgLookup, cfg.BaseHost))
	r.Use(middleware.RLSContext(middleware.NewBunRunner(appDB)))

	r.Get("/api/v1/health", handler.Health())

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// redactDSN returns the input DSN with the password component blanked.
func redactDSN(s string) string {
	at := -1
	for i := range s {
		if s[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return s
	}
	count := 0
	colon := -1
	for i := 0; i < at; i++ {
		if s[i] == ':' {
			count++
			if count == 2 {
				colon = i
				break
			}
		}
	}
	if colon < 0 {
		return s
	}
	return s[:colon+1] + "***" + s[at:]
}
