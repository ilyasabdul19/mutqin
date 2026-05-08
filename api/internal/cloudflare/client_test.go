// api/internal/cloudflare/client_test.go
package cloudflare_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ilyas/mutqin-api/internal/cloudflare"
)

type stubServer struct {
	method string
	path   string
	body   string
	status int
	resp   string
}

func newStubServer(t *testing.T, status int, resp string) (*httptest.Server, *stubServer) {
	t.Helper()
	stub := &stubServer{status: status, resp: resp}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.method = r.Method
		stub.path = r.URL.Path
		buf, _ := io.ReadAll(r.Body)
		stub.body = string(buf)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(stub.status)
		_, _ = w.Write([]byte(stub.resp))
	}))
	t.Cleanup(srv.Close)
	return srv, stub
}

func TestCreateProxiedARecord_HappyPath(t *testing.T) {
	srv, stub := newStubServer(t, http.StatusOK, `{
		"success": true,
		"errors": [],
		"messages": [],
		"result": {"id": "rec-123", "name": "alfalah.mutqin.app", "type": "A", "content": "1.2.3.4", "proxied": true}
	}`)

	c := cloudflare.New("token", "zone-1", srv.URL)
	id, err := c.CreateProxiedARecord(context.Background(), "alfalah.mutqin.app", "1.2.3.4")
	if err != nil {
		t.Fatalf("CreateProxiedARecord: %v", err)
	}
	if id != "rec-123" {
		t.Fatalf("got id %q, want rec-123", id)
	}
	if stub.method != http.MethodPost {
		t.Fatalf("got method %s, want POST", stub.method)
	}
	if !strings.HasSuffix(stub.path, "/zones/zone-1/dns_records") {
		t.Fatalf("got path %q, want suffix /zones/zone-1/dns_records", stub.path)
	}

	var sentBody map[string]any
	if err := json.Unmarshal([]byte(stub.body), &sentBody); err != nil {
		t.Fatalf("parse sent body: %v (raw=%s)", err, stub.body)
	}
	if sentBody["type"] != "A" {
		t.Fatalf("type=%v, want A", sentBody["type"])
	}
	if sentBody["name"] != "alfalah.mutqin.app" {
		t.Fatalf("name=%v, want alfalah.mutqin.app", sentBody["name"])
	}
	if sentBody["content"] != "1.2.3.4" {
		t.Fatalf("content=%v, want 1.2.3.4", sentBody["content"])
	}
	if sentBody["proxied"] != true {
		t.Fatalf("proxied=%v, want true", sentBody["proxied"])
	}
}

func TestCreateProxiedARecord_AlreadyExists_Idempotent(t *testing.T) {
	srv, _ := newStubServer(t, http.StatusBadRequest, `{
		"success": false,
		"errors": [{"code": 81057, "message": "Record already exists."}],
		"messages": [],
		"result": null
	}`)

	c := cloudflare.New("token", "zone-1", srv.URL)
	id, err := c.CreateProxiedARecord(context.Background(), "alfalah.mutqin.app", "1.2.3.4")
	if err != nil {
		t.Fatalf("CreateProxiedARecord: want nil err on already-exists, got %v", err)
	}
	if id != "" {
		t.Fatalf("got id %q on already-exists, want empty", id)
	}
}

func TestCreateProxiedARecord_OtherError(t *testing.T) {
	srv, _ := newStubServer(t, http.StatusForbidden, `{
		"success": false,
		"errors": [{"code": 9109, "message": "Unauthorized to access requested resource."}],
		"messages": [],
		"result": null
	}`)

	c := cloudflare.New("token", "zone-1", srv.URL)
	_, err := c.CreateProxiedARecord(context.Background(), "alfalah.mutqin.app", "1.2.3.4")
	if err == nil {
		t.Fatal("want error on 403 unauthorized")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("error=%v, want to mention Unauthorized", err)
	}
}

func TestCreateProxiedARecord_AuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"errors":[],"messages":[],"result":{"id":"x"}}`))
	}))
	t.Cleanup(srv.Close)

	c := cloudflare.New("the-token", "zone-1", srv.URL)
	if _, err := c.CreateProxiedARecord(context.Background(), "x.mutqin.app", "1.2.3.4"); err != nil {
		t.Fatalf("call: %v", err)
	}
	if gotAuth != "Bearer the-token" {
		t.Fatalf("Authorization=%q, want Bearer the-token", gotAuth)
	}
}
