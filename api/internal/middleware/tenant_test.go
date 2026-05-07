// api/internal/middleware/tenant_test.go
package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

type fakeLookup struct {
	bySlug map[string]uuid.UUID
}

func (f fakeLookup) GetOrgIDBySlug(_ context.Context, slug string) (uuid.UUID, error) {
	if id, ok := f.bySlug[slug]; ok {
		return id, nil
	}
	return uuid.Nil, errors.New("not found")
}

func TestTenantResolver_FromHeader(t *testing.T) {
	id := uuid.New()
	mw := middleware.Tenant(fakeLookup{bySlug: map[string]uuid.UUID{"alfalah": id}}, "mutqin.app")
	var got uuid.UUID
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-Slug", "alfalah")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok {
		t.Fatal("expected tenant in ctx")
	}
	if got != id {
		t.Fatalf("got %s, want %s", got, id)
	}
}

func TestTenantResolver_FromHost(t *testing.T) {
	id := uuid.New()
	mw := middleware.Tenant(fakeLookup{bySlug: map[string]uuid.UUID{"noor": id}}, "mutqin.app")
	var got uuid.UUID
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "noor.mutqin.app"
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !ok || got != id {
		t.Fatalf("got=(%v,%v), want (%s,true)", got, ok, id)
	}
}

func TestTenantResolver_HeaderWinsOverHost(t *testing.T) {
	header := uuid.New()
	hostID := uuid.New()
	mw := middleware.Tenant(fakeLookup{bySlug: map[string]uuid.UUID{
		"from-header": header,
		"from-host":   hostID,
	}}, "mutqin.app")
	var got uuid.UUID
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "from-host.mutqin.app"
	req.Header.Set("X-Tenant-Slug", "from-header")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != header {
		t.Fatalf("got %s, want %s (header should win)", got, header)
	}
}

func TestTenantResolver_NoSlugLeavesCtxUnchanged(t *testing.T) {
	mw := middleware.Tenant(fakeLookup{}, "mutqin.app")
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "mutqin.app" // apex
	h.ServeHTTP(httptest.NewRecorder(), req)

	if ok {
		t.Fatal("expected no tenant in ctx for apex host with no header")
	}
}

func TestTenantResolver_UnknownSlugReturns404(t *testing.T) {
	mw := middleware.Tenant(fakeLookup{}, "mutqin.app")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-Slug", "ghost")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if called {
		t.Fatal("handler should not be invoked when slug unknown")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}
