// api/internal/middleware/auth_as_tenant_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestAuthAsTenant_PromotesIdentityToTenant(t *testing.T) {
	mw := middleware.AuthAsTenant
	orgID := uuid.New()
	var got uuid.UUID
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !ok || got != orgID {
		t.Fatalf("got=(%v,%v) want (%s,true)", got, ok, orgID)
	}
}

func TestAuthAsTenant_DoesNotOverrideExistingTenant(t *testing.T) {
	hostOrg := uuid.New()
	authOrg := uuid.New()
	mw := middleware.AuthAsTenant

	var got uuid.UUID
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := tenant.With(req.Context(), hostOrg)
	ctx = auth.With(ctx, auth.Identity{UserID: uuid.New(), OrgID: &authOrg, Role: "center_admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got != hostOrg {
		t.Fatalf("got %s want hostOrg %s (existing tenant should not be overridden)", got, hostOrg)
	}
}

func TestAuthAsTenant_PassesThroughForSuperAdmin(t *testing.T) {
	mw := middleware.AuthAsTenant
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: nil, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ok {
		t.Fatal("super_admin should not have a tenant in ctx")
	}
}
