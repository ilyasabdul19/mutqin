package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE sync_conflicts (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    table_name TEXT NOT NULL,
			    record_id UUID NOT NULL,
			    client_id TEXT,
			    client_data JSONB NOT NULL,
			    server_data JSONB NOT NULL,
			    resolution TEXT NOT NULL DEFAULT 'last_write_wins',
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_sync_conflicts_org_created ON sync_conflicts(organization_id, created_at DESC)`,
			`GRANT SELECT, INSERT ON sync_conflicts TO mutqin_app`,
			// sync_conflicts IS tenant-scoped but writes happen during conflict
			// resolution and need ctx-tenant set. Enable RLS for safety.
			`ALTER TABLE sync_conflicts ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE sync_conflicts FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON sync_conflicts
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
			`DROP POLICY IF EXISTS tenant_isolation ON sync_conflicts`,
			`ALTER TABLE sync_conflicts NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE sync_conflicts DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE sync_conflicts`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
