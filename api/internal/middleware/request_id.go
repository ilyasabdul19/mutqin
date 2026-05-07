// api/internal/middleware/request_id.go
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey struct{}

const requestIDHeader = "X-Request-ID"

// RequestID returns middleware that ensures every request carries a stable
// request ID. The ID comes from the incoming X-Request-ID header if present,
// otherwise a fresh UUID is generated. The ID is placed in ctx and echoed back
// in the response headers so clients and downstream services see the same one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored by RequestID middleware,
// or "" if not present.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}
