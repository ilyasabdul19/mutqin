package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE halaqat (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    name TEXT NOT NULL,
			    teacher_id UUID REFERENCES users(id),
			    schedule JSONB,
			    max_capacity INT NOT NULL DEFAULT 30,
			    status TEXT NOT NULL DEFAULT 'active'
			      CHECK (status IN ('active', 'inactive')),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_halaqat_organization_id ON halaqat(organization_id)`,
			`CREATE INDEX idx_halaqat_teacher_id ON halaqat(teacher_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON halaqat TO mutqin_app`,
			`ALTER TABLE halaqat ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE halaqat FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON halaqat
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
			`DROP POLICY IF EXISTS tenant_isolation ON halaqat`,
			`ALTER TABLE halaqat NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE halaqat DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE halaqat`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
