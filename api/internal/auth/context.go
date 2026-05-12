// api/internal/auth/context.go
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Identity is what AuthMiddleware extracts from the JWT and places in ctx.
type Identity struct {
	UserID uuid.UUID
	OrgID  *uuid.UUID // nil for super_admin
	Role   string     // "super_admin" | "center_admin" | "teacher"
}

type ctxKey struct{}

// With returns a child ctx carrying the resolved Identity.
func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the Identity from ctx and ok=true if present.
func From(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(ctxKey{}).(Identity)
	return v, ok
}
