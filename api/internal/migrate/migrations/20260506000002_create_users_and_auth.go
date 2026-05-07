// api/internal/migrate/migrations/20260506000002_create_users_and_auth.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE users (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    phone TEXT,
			    email TEXT,
			    name TEXT NOT NULL,
			    role TEXT NOT NULL CHECK (role IN ('super_admin', 'center_admin', 'teacher')),
			    organization_id UUID REFERENCES organizations(id),
			    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'pending')),
			    language TEXT NOT NULL DEFAULT 'ar',
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE TABLE invites (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    role TEXT NOT NULL CHECK (role IN ('center_admin', 'teacher')),
			    token TEXT NOT NULL UNIQUE,
			    expires_at TIMESTAMPTZ NOT NULL,
			    used_at TIMESTAMPTZ,
			    created_by UUID NOT NULL REFERENCES users(id),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE TABLE otp_codes (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    email TEXT NOT NULL,
			    code TEXT NOT NULL,
			    expires_at TIMESTAMPTZ NOT NULL,
			    used BOOLEAN NOT NULL DEFAULT false,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_users_organization_id ON users(organization_id)`,
			`CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL`,
			`CREATE INDEX idx_invites_token ON invites(token)`,
			`CREATE INDEX idx_invites_organization_id ON invites(organization_id)`,
			`CREATE INDEX idx_otp_codes_email ON otp_codes(email)`,
			`ALTER TABLE users ADD CONSTRAINT users_org_role_check
			   CHECK (
			     (role = 'super_admin' AND organization_id IS NULL) OR
			     (role IN ('center_admin', 'teacher') AND organization_id IS NOT NULL)
			   )`,
			`ALTER TABLE users ADD CONSTRAINT users_contact_check
			   CHECK (phone IS NOT NULL OR email IS NOT NULL)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP TABLE otp_codes`,
			`DROP TABLE invites`,
			`DROP TABLE users`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
