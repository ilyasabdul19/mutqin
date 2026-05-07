// api/internal/db/hooks.go
package db

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// TenantHook is a Bun QueryHook that mutates the query's IQuery to add
// "WHERE organization_id = ?" whenever the request context carries a resolved
// tenant. It is the historical entry point for tenant-aware filtering and is
// kept here so callers can install it for visibility / future-proofing.
//
// IMPORTANT — actual SQL injection is performed by the per-model hooks on
// model.TenantScoped (BeforeSelect / BeforeUpdate / BeforeDelete). In
// Bun v1.2.x, *bun.QueryHook.BeforeQuery runs AFTER the SQL has already been
// rendered, so mutations to e.IQuery here do not affect the executed SQL.
// The per-model BeforeSelectHook etc. run BEFORE rendering and therefore can
// (and do) influence the final query.
//
// Net effect: tenant narrowing for any table backed by a model that embeds
// model.TenantScoped (e.g. User, Invite). Tables whose models do NOT embed
// it (e.g. Organization) remain unaffected — which is desired for the
// org-resolution lookup path (GetBySlugAdmin) and admin / setup paths.
//
// Bypass: omit the tenant from ctx (admin / migration / setup paths).
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
