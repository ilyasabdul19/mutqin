// api/internal/model/tenant_scoped.go
package model

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// TenantScoped is an embeddable marker that wires the per-model Bun hooks
// (BeforeSelect / BeforeUpdate / BeforeDelete) to inject
// `WHERE organization_id = $tenant` whenever the request ctx carries a
// resolved tenant.
//
// Bun's global QueryHook (db.TenantHook) cannot mutate SQL because it runs
// AFTER the query is rendered. Per-model hooks run BEFORE rendering, which
// is the only public extension point in Bun v1.2.x where a Where clause can
// still influence the final SQL.
//
// Embed this in any struct whose backing table has an `organization_id`
// column. Models without that column (e.g. Organization itself) simply do
// NOT embed it — they remain unaffected, which is what we want for
// org-resolution lookups (GetBySlugAdmin) and admin paths.
//
// The hook never adds a filter when the ctx has no tenant, so admin /
// migration / setup callers continue to operate cross-tenant.
type TenantScoped struct{}

// BeforeSelect injects organization_id filter from ctx.
func (TenantScoped) BeforeSelect(ctx context.Context, q *bun.SelectQuery) error {
	if orgID, ok := tenant.From(ctx); ok {
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return nil
}

// BeforeUpdate injects organization_id filter from ctx.
func (TenantScoped) BeforeUpdate(ctx context.Context, q *bun.UpdateQuery) error {
	if orgID, ok := tenant.From(ctx); ok {
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return nil
}

// BeforeDelete injects organization_id filter from ctx.
func (TenantScoped) BeforeDelete(ctx context.Context, q *bun.DeleteQuery) error {
	if orgID, ok := tenant.From(ctx); ok {
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return nil
}
