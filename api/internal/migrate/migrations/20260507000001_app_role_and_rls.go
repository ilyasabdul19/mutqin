// api/internal/migrate/migrations/20260507000001_app_role_and_rls.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			// Idempotent role creation. NOLOGIN until the migration explicitly grants login.
			`DO $$
			 BEGIN
			   IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mutqin_app') THEN
			     CREATE ROLE mutqin_app LOGIN PASSWORD 'mutqin_app';
			   END IF;
			 END $$`,
			// Schema usage + DML on existing tables.
			`GRANT USAGE ON SCHEMA public TO mutqin_app`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO mutqin_app`,
			`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO mutqin_app`,
			// Future tables get the same grants automatically.
			`ALTER DEFAULT PRIVILEGES IN SCHEMA public
			   GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO mutqin_app`,
			`ALTER DEFAULT PRIVILEGES IN SCHEMA public
			   GRANT USAGE, SELECT ON SEQUENCES TO mutqin_app`,
			// Enable + FORCE RLS so even table owners are subject (the role we run
			// as today, mutqin, owns the tables and would otherwise bypass policies).
			`ALTER TABLE users ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE users FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE invites ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE invites FORCE ROW LEVEL SECURITY`,
			// Policy: only see rows whose organization_id matches the per-request
			// app.current_tenant GUC. No tenant set => no rows visible.
			`CREATE POLICY tenant_isolation ON users
			   USING (organization_id = current_setting('app.current_tenant', true)::uuid)
			   WITH CHECK (organization_id = current_setting('app.current_tenant', true)::uuid)`,
			`CREATE POLICY tenant_isolation ON invites
			   USING (organization_id = current_setting('app.current_tenant', true)::uuid)
			   WITH CHECK (organization_id = current_setting('app.current_tenant', true)::uuid)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP POLICY IF EXISTS tenant_isolation ON invites`,
			`DROP POLICY IF EXISTS tenant_isolation ON users`,
			`ALTER TABLE invites NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE invites DISABLE ROW LEVEL SECURITY`,
			`ALTER TABLE users NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE users DISABLE ROW LEVEL SECURITY`,
			`REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM mutqin_app`,
			`REVOKE ALL ON ALL TABLES IN SCHEMA public FROM mutqin_app`,
			`REVOKE USAGE ON SCHEMA public FROM mutqin_app`,
			`DROP ROLE IF EXISTS mutqin_app`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
