// api/internal/migrate/runner.go
package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// Up applies all pending migrations.
func Up(ctx context.Context, db *bun.DB) error {
	migrator := migrate.NewMigrator(db, Migrations)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := migrator.Lock(ctx); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			slog.Error("unlock migrator", "error", err)
		}
	}()

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if group.IsZero() {
		slog.Info("no new migrations to apply")
		return nil
	}
	slog.Info("migrations applied", "group_id", group.ID, "count", len(group.Migrations))
	return nil
}

// Down rolls back the most recent migration group.
func Down(ctx context.Context, db *bun.DB) error {
	migrator := migrate.NewMigrator(db, Migrations)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := migrator.Lock(ctx); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			slog.Error("unlock migrator", "error", err)
		}
	}()

	group, err := migrator.Rollback(ctx)
	if err != nil {
		return fmt.Errorf("rollback migrations: %w", err)
	}
	if group.IsZero() {
		slog.Info("no migrations to roll back")
		return nil
	}
	slog.Info("migrations rolled back", "group_id", group.ID, "count", len(group.Migrations))
	return nil
}
