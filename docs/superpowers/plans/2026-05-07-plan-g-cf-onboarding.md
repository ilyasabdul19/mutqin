# Plan G — Cloudflare API Tenant Onboarding

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Cloudflare API client + an `onboard` CLI subcommand that creates a proxied A record `<slug>.mutqin.app` → VPS IP via the Cloudflare API. The package is the seam; the CLI is the manual entry point for the existing 50 tenants. A future Super Admin endpoint will call the same package when a center is created in-app.

**Architecture:** A small `internal/cloudflare` package owns the HTTP client. It exposes `Client.CreateProxiedARecord(ctx, name, ip)` which POSTs to `/zones/{zone}/dns_records` with `proxied=true`, idempotent (returns success if a matching record already exists). The client is constructed from env vars (`CF_DNS_API_TOKEN`, `CF_ZONE_ID`, `CF_RECORD_TARGET_IP`). Tests run against a stub server (`httptest.NewServer`) covering the create + already-exists paths. A new `cmd/onboard/main.go` reads `--slug` from flags, builds the client, and prints the result.

**Tech Stack:**
- New: stdlib `net/http`, `encoding/json`
- Existing: `slog`, `flag`
- External (production only): Cloudflare API v4

**Out of scope (deferred):**
- Wiring the package into a Super Admin org-create handler (no such handler yet — separate epic)
- Bulk onboard of all existing orgs (deferred — for now operators run `onboard <slug>` once per existing tenant)
- Record cleanup on tenant deletion (separate small follow-up)
- Backups to R2 (mentioned in `docs/deploy.md` — separate plan)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/cloudflare/client.go` | HTTP client + `CreateProxiedARecord` method |
| `api/internal/cloudflare/client_test.go` | Tests against `httptest` stub |
| `api/cmd/onboard/main.go` | CLI: `onboard --slug=<slug>` |
| `api/Dockerfile.onboard` | Optional one-shot image (small, runs CLI) |

### Modified

| Path | Change |
|------|--------|
| `api/internal/config/config.go` | Add `CFAPIToken`, `CFZoneID`, `CFRecordTargetIP`, `CFRecordBaseDomain` fields with env loading + sane defaults |
| `docs/deploy.md` | New "Onboarding a tenant" section |

### Deleted
None.

---

## Tasks

### Task 1: TDD Cloudflare client — failing tests

**Files:**
- Create: `api/internal/cloudflare/client_test.go`

- [ ] **Step 1: Write the tests**

```go
// api/internal/cloudflare/client_test.go
package cloudflare_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ilyas/mutqin-api/internal/cloudflare"
)

// stubServer captures the last received request and returns the configured response.
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
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
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
	// Cloudflare returns 400 with code 81057 when a record with the same name+type
	// already exists. Treat as success.
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
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g/api && go test ./internal/cloudflare/...
```

Expected: `package internal/cloudflare: no Go files`.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g add api/internal/cloudflare/client_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g commit -m "test: failing Cloudflare client tests"
```

---

### Task 2: Implement the Cloudflare client

**Files:**
- Create: `api/internal/cloudflare/client.go`

- [ ] **Step 1: Write the client**

```go
// api/internal/cloudflare/client.go
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the public Cloudflare v4 API root. Override in tests.
const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

// errorAlreadyExists is the Cloudflare error code returned when a record with
// the same name + type already exists.
const errorAlreadyExists = 81057

// Client talks to the Cloudflare API.
type Client struct {
	token   string
	zoneID  string
	baseURL string
	http    *http.Client
}

// New constructs a Client. baseURL is usually DefaultBaseURL; tests pass an
// httptest server URL.
func New(token, zoneID, baseURL string) *Client {
	return &Client{
		token:   token,
		zoneID:  zoneID,
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// envelope is Cloudflare's standard response shape.
type envelope struct {
	Success  bool             `json:"success"`
	Errors   []envelopeError  `json:"errors"`
	Messages []envelopeError  `json:"messages"`
	Result   *json.RawMessage `json:"result"`
}

type envelopeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type recordResult struct {
	ID string `json:"id"`
}

// CreateProxiedARecord creates a proxied A record `name → ip` in the configured
// zone. Returns the new record's ID on success. If a record with the same
// name+type already exists, returns ("", nil) — idempotent.
func (c *Client) CreateProxiedARecord(ctx context.Context, name, ip string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"type":    "A",
		"name":    name,
		"content": ip,
		"ttl":     1, // 1 = automatic
		"proxied": true,
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records", c.baseURL, c.zoneID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return "", fmt.Errorf("decode response: %w (status=%d)", err, resp.StatusCode)
	}

	if env.Success {
		if env.Result == nil {
			return "", errors.New("cloudflare: success but missing result")
		}
		var rec recordResult
		if err := json.Unmarshal(*env.Result, &rec); err != nil {
			return "", fmt.Errorf("unmarshal result: %w", err)
		}
		return rec.ID, nil
	}

	// Idempotent: treat already-exists as a no-op success.
	for _, e := range env.Errors {
		if e.Code == errorAlreadyExists {
			return "", nil
		}
	}

	if len(env.Errors) > 0 {
		return "", fmt.Errorf("cloudflare error %d: %s", env.Errors[0].Code, env.Errors[0].Message)
	}
	return "", fmt.Errorf("cloudflare request failed (status=%d)", resp.StatusCode)
}
```

- [ ] **Step 2: Run tests, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g/api && go test ./internal/cloudflare/... -v 2>&1 | tail -15
```

Expected: 4 tests PASS.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g add api/internal/cloudflare/client.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g commit -m "cloudflare: client with idempotent CreateProxiedARecord"
```

---

### Task 3: Add CF env vars to config

**Files:**
- Modify: `api/internal/config/config.go`

- [ ] **Step 1: Open the file and add the fields + loading**

Add to the `Config` struct (after the existing R2 fields):

```go
	CFAPIToken         string
	CFZoneID           string
	CFRecordTargetIP   string
	CFRecordBaseDomain string
```

In `Load()`, after the existing assignments and before the `if cfg.DatabaseURL == ""` validation block, add:

```go
	cfg.CFAPIToken = os.Getenv("CF_DNS_API_TOKEN")
	cfg.CFZoneID = os.Getenv("CF_ZONE_ID")
	cfg.CFRecordTargetIP = os.Getenv("CF_RECORD_TARGET_IP")
	cfg.CFRecordBaseDomain = os.Getenv("CF_RECORD_BASE_DOMAIN")
	if cfg.CFRecordBaseDomain == "" {
		cfg.CFRecordBaseDomain = "mutqin.app"
	}
```

Note: these are NOT marked required in `config.Load()`. The api/landing binaries don't need them; only `cmd/onboard` does. The onboard CLI validates presence itself.

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g/api && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g add api/internal/config/config.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g commit -m "config: CF_DNS_API_TOKEN, CF_ZONE_ID, CF_RECORD_TARGET_IP, CF_RECORD_BASE_DOMAIN"
```

---

### Task 4: `cmd/onboard/main.go`

**Files:**
- Create: `api/cmd/onboard/main.go`

- [ ] **Step 1: Write the CLI**

```go
// api/cmd/onboard/main.go
//
// onboard creates a proxied Cloudflare A record for a tenant slug.
// Usage:
//   onboard --slug=alfalah
//
// Reads CF_DNS_API_TOKEN, CF_ZONE_ID, CF_RECORD_TARGET_IP, CF_RECORD_BASE_DOMAIN
// from env. Idempotent — re-running is safe.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/ilyas/mutqin-api/internal/cloudflare"
	"github.com/ilyas/mutqin-api/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slug := flag.String("slug", "", "tenant slug (lowercase, alphanumeric + dash)")
	flag.Parse()

	if *slug == "" {
		slog.Error("--slug is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	if cfg.CFAPIToken == "" || cfg.CFZoneID == "" || cfg.CFRecordTargetIP == "" {
		slog.Error("missing Cloudflare config",
			"have_token", cfg.CFAPIToken != "",
			"have_zone", cfg.CFZoneID != "",
			"have_target_ip", cfg.CFRecordTargetIP != "",
		)
		os.Exit(1)
	}

	name := fmt.Sprintf("%s.%s", *slug, cfg.CFRecordBaseDomain)
	c := cloudflare.New(cfg.CFAPIToken, cfg.CFZoneID, cloudflare.DefaultBaseURL)

	id, err := c.CreateProxiedARecord(context.Background(), name, cfg.CFRecordTargetIP)
	if err != nil {
		slog.Error("create record", "name", name, "error", err)
		os.Exit(1)
	}
	if id == "" {
		slog.Info("record already exists (no-op)", "name", name)
	} else {
		slog.Info("record created", "name", name, "id", id, "target", cfg.CFRecordTargetIP)
	}
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g/api && go build ./cmd/onboard
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g add api/cmd/onboard/main.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g commit -m "cmd: onboard CLI for Cloudflare per-tenant DNS"
```

---

### Task 5: Update `docs/deploy.md` with onboarding section

**Files:**
- Modify: `docs/deploy.md`

- [ ] **Step 1: Append a new section at the bottom**

```markdown
## Onboarding a tenant

When a new center is created, the operator (or a future Super Admin handler) must create a Cloudflare DNS record so the tenant's subdomain (`<slug>.mutqin.app`) routes to the VPS through Cloudflare's CDN.

### Required env vars (in addition to the deploy ones above)

```
CF_DNS_API_TOKEN=<same token used for ACME — Zone:DNS:Edit on mutqin.app>
CF_ZONE_ID=<the zone id; find via: GET https://api.cloudflare.com/client/v4/zones?name=mutqin.app>
CF_RECORD_TARGET_IP=<VPS public IP>
CF_RECORD_BASE_DOMAIN=mutqin.app  # default; override only for staging
```

### Run the onboard CLI

From inside the api container (which has the binary baked in):

```sh
docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file /opt/mutqin/.env exec api /app/onboard --slug=<slug>
```

Or with a one-shot container if the api image doesn't include the onboard binary:

```sh
docker run --rm \
  -e CF_DNS_API_TOKEN=$CF_DNS_API_TOKEN \
  -e CF_ZONE_ID=$CF_ZONE_ID \
  -e CF_RECORD_TARGET_IP=$CF_RECORD_TARGET_IP \
  $CI_REGISTRY_IMAGE/onboard:latest --slug=<slug>
```

Idempotency: if the record already exists, the CLI exits 0 with a `record already exists (no-op)` log line.
```

- [ ] **Step 2: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g add docs/deploy.md
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g commit -m "docs(deploy): tenant onboarding section (CF DNS via onboard CLI)"
```

---

### Task 6: Final verification + push

- [ ] **Step 1: Run the full test suite**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g/api && go clean -testcache && go test ./... 2>&1 | tail -10
```

Expected: all PASS, including new `internal/cloudflare` tests.

- [ ] **Step 2: vet**

```bash
go vet ./...
```

- [ ] **Step 3: Push to both remotes**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g push -u origin feat/plan-g-cf-onboarding
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-g push gitlab feat/plan-g-cf-onboarding
```

- [ ] **Step 4: Open PR + MR**

Base: `docs/backend-architecture-v2`.

Title: `Plan G — Cloudflare API tenant onboarding`

Body:

```
## Summary
- New `internal/cloudflare` package with idempotent `CreateProxiedARecord` (treats CF error code 81057 "already exists" as success).
- 4 tests against an `httptest` stub: happy path, already-exists, other error, auth header.
- New `cmd/onboard --slug=<slug>` CLI — reads env, calls CF, logs result.
- Config gains `CF_DNS_API_TOKEN`, `CF_ZONE_ID`, `CF_RECORD_TARGET_IP`, `CF_RECORD_BASE_DOMAIN` (last defaults to `mutqin.app`).
- `docs/deploy.md` extended with an "Onboarding a tenant" section.

## Verification
- [x] `go test ./internal/cloudflare/...` — 4 PASS.
- [x] `go test ./...` clean.
- [x] `go vet ./...` clean.
- [x] Real CF API not exercised — tests use httptest stubs (correct seam: CF call is in production deploy step only).

## Out of scope (future)
- Wire CF onboarding into a Super Admin org-create handler (no such handler yet).
- Bulk onboard of existing tenants.
- Record cleanup on tenant deletion.
```

---

## Self-Review

**Spec coverage:**
- D5 (CF Free + auto-create proxied A records via CF API) — Tasks 1–4 ✓
- Required-changes #9 (Cloudflare API onboarding utility in `api/cmd/onboard/`) — Task 4 ✓
- Risk-table entry "Per-tenant explicit A record auto-created via CF API on tenant creation; falls back to gray-cloud if API call fails" — partially: this plan ships the create path. Alert/fallback is operator-driven (CLI failure surfaces via stderr/exit-1 in a CI deploy step or manual run).

**Placeholder scan:** No "TBD"/"TODO" / "implement later". Every step gives exact code or commands.

**Type / signature consistency:**
- `cloudflare.New(token, zoneID, baseURL string) *Client` and `Client.CreateProxiedARecord(ctx, name, ip string) (string, error)` consistent across Tasks 1, 2, 4.
- Config field names (`CFAPIToken`, `CFZoneID`, `CFRecordTargetIP`, `CFRecordBaseDomain`) consistent in `config.go` (Task 3) and `cmd/onboard/main.go` (Task 4).
- Env var names (`CF_DNS_API_TOKEN`, `CF_ZONE_ID`, `CF_RECORD_TARGET_IP`, `CF_RECORD_BASE_DOMAIN`) consistent in config, CLI, and `docs/deploy.md`.
- `CF_DNS_API_TOKEN` deliberately reuses the same env var name Plan D set for ACME, since the same scope (`Zone:DNS:Edit`) covers both — one secret, two consumers.

**Scope:** 6 tasks. Substantive: 1, 2 (TDD pair). Trivial: 3, 4, 5, 6.
