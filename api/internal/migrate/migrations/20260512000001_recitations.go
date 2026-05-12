package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE recitations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    student_id UUID NOT NULL REFERENCES students(id),
			    halaqah_id UUID NOT NULL REFERENCES halaqat(id),
			    teacher_id UUID NOT NULL REFERENCES users(id),
			    type TEXT NOT NULL CHECK (type IN ('new_hifz', 'near_review', 'far_review')),
			    surah_number INT NOT NULL CHECK (surah_number BETWEEN 1 AND 114),
			    ayah_from INT NOT NULL CHECK (ayah_from >= 1),
			    ayah_to INT NOT NULL CHECK (ayah_to >= ayah_from),
			    grade TEXT NOT NULL CHECK (grade IN ('mumtaz', 'jayyid_jiddan', 'jayyid', 'maqbul', 'daif')),
			    notes TEXT,
			    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			    synced_at TIMESTAMPTZ,
			    client_id TEXT
			)`,
			`CREATE INDEX idx_recitations_student_recorded
			   ON recitations(student_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_halaqah_recorded
			   ON recitations(halaqah_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_organization_id
			   ON recitations(organization_id)`,
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
