package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE announcements (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    title TEXT NOT NULL,
			    body TEXT NOT NULL,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_announcements_org_created_at
			   ON announcements(organization_id, created_at DESC)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON announcements TO mutqin_app`,
			`ALTER TABLE announcements ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE announcements FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON announcements
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
			`DROP POLICY IF EXISTS tenant_isolation ON announcements`,
			`ALTER TABLE announcements NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE announcements DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE announcements`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
