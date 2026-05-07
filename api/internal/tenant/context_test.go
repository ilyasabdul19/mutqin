// api/internal/tenant/context_test.go
package tenant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestWithAndFrom_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := tenant.With(context.Background(), id)

	got, ok := tenant.From(ctx)
	if !ok {
		t.Fatal("From returned ok=false on a populated ctx")
	}
	if got != id {
		t.Fatalf("got %s, want %s", got, id)
	}
}

func TestFrom_EmptyCtx(t *testing.T) {
	_, ok := tenant.From(context.Background())
	if ok {
		t.Fatal("From returned ok=true on an empty ctx")
	}
}
