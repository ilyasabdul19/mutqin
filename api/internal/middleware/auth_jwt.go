// api/internal/middleware/auth_jwt.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/ilyas/mutqin-api/internal/auth"
)

// Auth returns middleware that, when an Authorization: Bearer <jwt> header is
// present, validates the token via Verifier and stores the resulting Identity
// in ctx via auth.With. Bad tokens → 401. Absent header → pass through (so
// public routes work). Routes that require auth must compose Role(...) after
// Auth or check auth.From() themselves.
func Auth(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := v.Verify(tok)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := auth.With(r.Context(), auth.Identity{
				UserID: claims.UserID,
				OrgID:  claims.OrgID,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
