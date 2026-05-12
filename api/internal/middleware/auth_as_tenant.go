// api/internal/middleware/auth_as_tenant.go
package middleware

import (
	"net/http"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// AuthAsTenant promotes an authenticated user's organization id from the
// auth.Identity into tenant.With, so the existing RLSContext middleware
// downstream sees the tenant and runs SET LOCAL accordingly.
//
// Order of precedence: an explicit tenant set earlier (e.g., by the Tenant
// resolver from a host header) wins; this middleware only fills in when no
// tenant has been set yet AND the JWT belongs to a tenant-scoped role.
//
// super_admin requests intentionally pass through without a tenant — they
// operate cross-tenant on the admin handle, not via RLS.
func AuthAsTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if _, has := tenant.From(ctx); has {
			next.ServeHTTP(w, r)
			return
		}
		id, ok := auth.From(ctx)
		if !ok || id.OrgID == nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(tenant.With(ctx, *id.OrgID)))
	})
}
