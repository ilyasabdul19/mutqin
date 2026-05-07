// api/internal/db/hooks.go
package db

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// TenantHook auto-injects "WHERE organization_id = ?" on Select / Update / Delete
// queries whenever the request context carries a resolved tenant.
//
// Repo code MUST NOT add organization_id filters manually — this hook owns that
// concern. Bypass is achieved by NOT placing a tenant in the ctx (e.g., for
// admin/setup paths that intentionally cross tenants).
type TenantHook struct{}

var _ bun.QueryHook = TenantHook{}

func (TenantHook) BeforeQuery(ctx context.Context, e *bun.QueryEvent) context.Context {
	orgID, ok := tenant.From(ctx)
	if !ok {
		return ctx
	}
	switch q := e.IQuery.(type) {
	case *bun.SelectQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.UpdateQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.DeleteQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return ctx
}

func (TenantHook) AfterQuery(ctx context.Context, e *bun.QueryEvent) {}
