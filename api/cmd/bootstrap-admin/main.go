// api/cmd/bootstrap-admin/main.go
//
// bootstrap-admin creates a single super_admin user so the system has a way in
// before Plan I ships the proper invite flow.
//
// Usage:
//   bootstrap-admin --email=ops@mutqin.app --name="Ops"
//
// Idempotent: if a user with that email already exists, prints its id and exits 0.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	emailFlag := flag.String("email", "", "super admin email (required)")
	nameFlag := flag.String("name", "Super Admin", "display name")
	flag.Parse()

	if *emailFlag == "" {
		slog.Error("--email is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect db", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	users := repo.NewUserRepo(adminDB)

	if existing, err := users.GetByEmailGlobal(ctx, *emailFlag); err == nil {
		slog.Info("user already exists", "id", existing.ID, "email", *emailFlag)
		return
	} else if !errors.Is(err, repo.ErrNotFound) {
		slog.Error("lookup", "error", err)
		os.Exit(1)
	}

	u := &model.User{
		Name:     *nameFlag,
		Email:    emailFlag,
		Role:     "super_admin",
		Status:   "active",
		Language: "ar",
	}
	if err := users.Create(ctx, u); err != nil {
		slog.Error("create user", "error", err)
		os.Exit(1)
	}
	slog.Info("super_admin created", "id", u.ID, "email", *emailFlag)
}
