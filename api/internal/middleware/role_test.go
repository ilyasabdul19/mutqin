// api/internal/middleware/role_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestRole_AllowsMatchingRole(t *testing.T) {
	mw := middleware.Role("center_admin", "super_admin")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	ctx := auth.With(httptest.NewRequest(http.MethodGet, "/", nil).Context(), auth.Identity{
		UserID: uuid.New(), Role: "center_admin",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("matching role: handler should have been called")
	}
}

func TestRole_DeniesOtherRole(t *testing.T) {
	mw := middleware.Role("super_admin")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	ctx := auth.With(httptest.NewRequest(http.MethodGet, "/", nil).Context(), auth.Identity{
		UserID: uuid.New(), Role: "teacher",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Fatal("non-matching role: handler should not have been called")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403", rec.Code)
	}
}

func TestRole_NoIdentity_Returns401(t *testing.T) {
	mw := middleware.Role("teacher")
	h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", rec.Code)
	}
}
