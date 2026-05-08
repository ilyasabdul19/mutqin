// api/internal/middleware/role.go
package middleware

import (
	"net/http"

	"github.com/ilyas/mutqin-api/internal/auth"
)

// Role returns middleware that requires the request's Identity to have a role
// in the allowed list. Missing identity → 401. Wrong role → 403. Compose AFTER
// the Auth middleware.
func Role(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.From(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if _, ok := allowedSet[id.Role]; !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
