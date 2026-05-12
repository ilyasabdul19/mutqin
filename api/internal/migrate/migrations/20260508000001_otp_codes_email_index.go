// api/internal/migrate/migrations/20260508000001_otp_codes_email_index.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx,
			`CREATE UNIQUE INDEX idx_otp_codes_email_active
			   ON otp_codes(email) WHERE used = false`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx,
			`DROP INDEX IF EXISTS idx_otp_codes_email_active`)
		return err
	})
}
