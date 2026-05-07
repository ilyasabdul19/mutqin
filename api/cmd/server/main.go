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

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)

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

	bdb, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer bdb.Close()
	slog.Info("connected to database")

	switch {
	case *migrateUp:
		if err := migrate.Up(ctx, bdb); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
		return
	case *migrateDown:
		if err := migrate.Down(ctx, bdb); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := migrate.Up(ctx, bdb); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.CORS)
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
