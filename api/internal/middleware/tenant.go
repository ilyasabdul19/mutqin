// api/internal/middleware/tenant.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// OrgLookup resolves a tenant slug to its organization UUID.
type OrgLookup interface {
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
}

// Tenant returns middleware that resolves the tenant from the request and
// injects the organization UUID into ctx via tenant.With.
//
// Resolution priority: X-Tenant-Slug header, then Host subdomain (only if Host
// matches "<slug>.<baseHost>"). If a slug is found but maps to nothing, the
// request is rejected with 404. If no slug is present at all, the chain
// proceeds with an unscoped ctx (e.g. apex host or admin path).
func Tenant(lookup OrgLookup, baseHost string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := r.Header.Get("X-Tenant-Slug")
			if slug == "" {
				slug = slugFromHost(r.Host, baseHost)
			}
			if slug == "" {
				next.ServeHTTP(w, r)
				return
			}
			id, err := lookup.GetOrgIDBySlug(r.Context(), slug)
			if err != nil {
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
			ctx := tenant.With(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// slugFromHost returns the leftmost subdomain when host matches "*.baseHost".
// Returns "" for apex, base-host-only, or any non-matching host (e.g. localhost).
func slugFromHost(host, baseHost string) string {
	host = strings.ToLower(strings.Split(host, ":")[0])
	baseHost = strings.ToLower(baseHost)
	if host == baseHost {
		return ""
	}
	suffix := "." + baseHost
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	return strings.TrimSuffix(host, suffix)
}
