// api/internal/tenant/context.go
package tenant

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

// With returns a child ctx carrying the resolved tenant organization ID.
func With(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, orgID)
}

// From returns the tenant organization ID from ctx and ok=true if present.
func From(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return v, ok
}
