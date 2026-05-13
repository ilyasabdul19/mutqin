package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE attendance (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    halaqah_id UUID NOT NULL REFERENCES halaqat(id),
			    student_id UUID NOT NULL REFERENCES students(id),
			    date DATE NOT NULL,
			    status TEXT NOT NULL CHECK (status IN ('present', 'absent')),
			    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			    synced_at TIMESTAMPTZ,
			    client_id TEXT,
			    UNIQUE (student_id, date)
			)`,
			`CREATE INDEX idx_attendance_halaqah_date ON attendance(halaqah_id, date DESC)`,
			`CREATE INDEX idx_attendance_organization_id ON attendance(organization_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON attendance TO mutqin_app`,
			`ALTER TABLE attendance ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE attendance FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON attendance
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
			`DROP POLICY IF EXISTS tenant_isolation ON attendance`,
			`ALTER TABLE attendance NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE attendance DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE attendance`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
