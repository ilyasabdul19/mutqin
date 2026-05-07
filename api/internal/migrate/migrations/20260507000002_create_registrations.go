// api/internal/migrate/migrations/20260507000002_create_registrations.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE registrations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    child_name TEXT NOT NULL,
			    child_age INT,
			    parent_phone TEXT,
			    parent_email TEXT,
			    hifz_level TEXT,
			    status TEXT NOT NULL DEFAULT 'pending'
			      CHECK (status IN ('pending', 'approved', 'rejected')),
			    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_registrations_organization_id
			   ON registrations(organization_id)`,
			`CREATE INDEX idx_registrations_status
			   ON registrations(status)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON registrations TO mutqin_app`,
			`ALTER TABLE registrations ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE registrations FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON registrations
			   USING (organization_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
			   WITH CHECK (organization_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP POLICY IF EXISTS tenant_isolation ON registrations`,
			`ALTER TABLE registrations NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE registrations DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE registrations`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
