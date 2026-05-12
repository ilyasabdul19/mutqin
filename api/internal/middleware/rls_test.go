// api/internal/middleware/rls_test.go
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

type fakeBun struct {
	beginCalls int
	setLocal   string
	committed  bool
	rolled     bool
}

func (f *fakeBun) RunInTx(ctx context.Context, _ *bun.IDB, fn func(ctx context.Context, tx middleware.RLSTx) error) error {
	f.beginCalls++
	tx := &fakeTx{parent: f}
	if err := fn(ctx, tx); err != nil {
		f.rolled = true
		return err
	}
	f.committed = true
	return nil
}

type fakeTx struct{ parent *fakeBun }

func (f *fakeTx) ExecContext(_ context.Context, query string, _ ...any) error {
	f.parent.setLocal = query
	return nil
}

func TestRLSContext_SetsLocal_WhenTenantPresent(t *testing.T) {
	stub := &fakeBun{}
	mw := middleware.RLSContext(stub)

	id := uuid.New()
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(tenant.With(req.Context(), id))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Fatal("handler not invoked")
	}
	if stub.beginCalls != 1 {
		t.Fatalf("RunInTx called %d times, want 1", stub.beginCalls)
	}
	if stub.setLocal == "" || stub.setLocal[:9] != "SET LOCAL" {
		t.Fatalf("setLocal stmt missing or wrong: %q", stub.setLocal)
	}
	if !stub.committed {
		t.Fatal("expected commit")
	}
}

func TestRLSContext_PassThrough_WhenNoTenant(t *testing.T) {
	stub := &fakeBun{}
	mw := middleware.RLSContext(stub)

	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Fatal("handler not invoked")
	}
	if stub.beginCalls != 0 {
		t.Fatalf("RunInTx called %d times, want 0", stub.beginCalls)
	}
}
