package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE students (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    halaqah_id UUID REFERENCES halaqat(id),
			    name TEXT NOT NULL,
			    age INT,
			    parent_phone TEXT,
			    parent_email TEXT,
			    hifz_level TEXT,
			    status TEXT NOT NULL DEFAULT 'active'
			      CHECK (status IN ('active', 'inactive')),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_students_organization_id ON students(organization_id)`,
			`CREATE INDEX idx_students_halaqah_id ON students(halaqah_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON students TO mutqin_app`,
			`ALTER TABLE students ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE students FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON students
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
			`DROP POLICY IF EXISTS tenant_isolation ON students`,
			`ALTER TABLE students NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE students DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE students`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
