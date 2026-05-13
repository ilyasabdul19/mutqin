package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

// recitations records each ayah-range recital a student delivers in a halaqah.
// The table backs Plan M's dashboard activity counters and is RLS-subject so
// each tenant's data is isolated by the same app.current_tenant GUC mechanism
// as the other student-bearing tables.
func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE recitations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    halaqah_id UUID NOT NULL REFERENCES halaqat(id),
			    student_id UUID NOT NULL REFERENCES students(id),
			    surah TEXT NOT NULL,
			    ayah_from INT NOT NULL,
			    ayah_to INT NOT NULL,
			    notes TEXT,
			    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_recitations_org_recorded ON recitations(organization_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_halaqah_recorded ON recitations(halaqah_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_student ON recitations(student_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON recitations TO mutqin_app`,
			`ALTER TABLE recitations ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE recitations FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON recitations
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
			`DROP POLICY IF EXISTS tenant_isolation ON recitations`,
			`ALTER TABLE recitations NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE recitations DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE recitations`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
