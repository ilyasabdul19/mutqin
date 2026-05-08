// api/internal/migrate/migrations/20260508000002_audit_log.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE audit_log (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    actor_id UUID REFERENCES users(id),
			    action TEXT NOT NULL,
			    target_type TEXT,
			    target_id UUID,
			    details JSONB,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_audit_log_actor_id ON audit_log(actor_id)`,
			`CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC)`,
			`GRANT SELECT, INSERT ON audit_log TO mutqin_app`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE audit_log`)
		return err
	})
}
