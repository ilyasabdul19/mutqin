// api/internal/middleware/auth_jwt_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestAuth_PopulatesIdentity(t *testing.T) {
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)

	uid := uuid.New()
	orgID := uuid.New()
	tok, _ := iss.Sign(auth.Claims{UserID: uid, OrgID: &orgID, Role: "teacher", TTL: time.Minute})

	var got auth.Identity
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !ok {
		t.Fatal("identity not set")
	}
	if got.UserID != uid {
		t.Fatalf("user_id: %s", got.UserID)
	}
	if got.OrgID == nil || *got.OrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if got.Role != "teacher" {
		t.Fatalf("role: %s", got.Role)
	}
}

func TestAuth_NoHeader_PassesThroughWithoutIdentity(t *testing.T) {
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = auth.From(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ok {
		t.Fatal("identity should not be present without Authorization header")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestAuth_BadToken_Returns401(t *testing.T) {
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", rec.Code)
	}
	if called {
		t.Fatal("handler should not have run")
	}
}
