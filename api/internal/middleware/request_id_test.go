// api/internal/middleware/request_id_test.go
package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var captured string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = middleware.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured == "" {
		t.Fatal("expected generated request id in ctx")
	}
	if rec.Header().Get("X-Request-ID") != captured {
		t.Fatalf("response header %q != ctx %q", rec.Header().Get("X-Request-ID"), captured)
	}
}

func TestRequestID_PropagatesIncoming(t *testing.T) {
	var captured string
	h := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = middleware.RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "incoming-id-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if captured != "incoming-id-123" {
		t.Fatalf("got %q, want incoming-id-123", captured)
	}
}

func TestRequestIDFromContext_AbsentReturnsEmpty(t *testing.T) {
	if got := middleware.RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}
