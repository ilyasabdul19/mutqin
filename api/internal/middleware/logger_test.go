// api/internal/middleware/logger_test.go
package middleware_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestLogger_EmitsRequestLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	chain := middleware.RequestID(middleware.Logger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", "req-abc")
	rec := httptest.NewRecorder()
	chain.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal log: %v (raw=%s)", err, buf.String())
	}
	if entry["msg"] != "http request" {
		t.Fatalf("msg=%v, want http request", entry["msg"])
	}
	if entry["request_id"] != "req-abc" {
		t.Fatalf("request_id=%v, want req-abc", entry["request_id"])
	}
	if entry["method"] != "GET" {
		t.Fatalf("method=%v, want GET", entry["method"])
	}
	if entry["path"] != "/test" {
		t.Fatalf("path=%v, want /test", entry["path"])
	}
	if entry["status"] != float64(200) {
		t.Fatalf("status=%v, want 200", entry["status"])
	}
}
