# Plan E — Landing Container (Go HTML Templates)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `landing` container — a separate Go binary serving server-rendered HTML for parent-facing pages (center landing + registration form). Routed by Traefik to `pages.localhost` (production: `pages.mutqin.app`). Includes the `registrations` table backing the form.

**Architecture:** A new `cmd/landing/main.go` binary reuses `internal/db`, `internal/migrate`, `internal/model`, `internal/repo`, and `internal/middleware` from the api binary, but mounts a different router built around Go `html/template`. Templates and CSS are bundled into the binary via `embed.FS` so the runtime image only contains a single distroless executable. The slug is taken from the URL path (`/{slug}`, `/{slug}/register`) — not the Host — so the existing tenant resolver doesn't apply; instead, a small per-request resolver looks up the org by slug, populates `tenant.From(ctx)`, and chains into the existing `RLSContext` middleware so RLS-subject INSERTs work correctly.

**Tech Stack:**
- New: `embed` (stdlib) for templates + static assets
- Existing: Bun ORM, slog, Chi (router for path matching), Postgres RLS, Traefik
- No JS / no React — plain HTML the parent's WhatsApp browser renders fast

**Out of scope (deferred):**
- Tenant theming on the landing page (separate Spec 2 brainstorm; this plan ships a generic neutral template)
- Email/SMS notification on new registration (deferred to messaging epic)
- Registration approval UI (already in Center Admin scope, Plan covers backend; UI is in `web`)
- GitLab CI building the landing image (Plan F)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/cmd/landing/main.go` | Landing binary entry — wires router, middleware, handlers, embedded templates |
| `api/internal/migrate/migrations/20260507000002_create_registrations.go` | Bun migration creating `registrations` table + RLS policy + GRANT to `mutqin_app` |
| `api/internal/model/registration.go` | Bun model (embeds `model.TenantScoped`) |
| `api/internal/repo/registration.go` | `RegistrationRepo` (Create + List) |
| `api/internal/repo/registration_test.go` | Tenant-narrowing integration tests |
| `api/internal/handler/landing/handler.go` | HTTP handlers for landing pages |
| `api/internal/handler/landing/templates/layout.html.tmpl` | Shared layout (`<html>`, `<head>`, RTL/LTR `dir`, viewport meta) |
| `api/internal/handler/landing/templates/center.html.tmpl` | Center landing page — name, description, schedule, registration link |
| `api/internal/handler/landing/templates/register.html.tmpl` | Registration form |
| `api/internal/handler/landing/templates/success.html.tmpl` | Post-submit success page |
| `api/internal/handler/landing/templates/not_found.html.tmpl` | 404 page (unknown slug) |
| `api/internal/handler/landing/static/style.css` | Minimal RTL/LTR-friendly CSS, ~60 lines |
| `api/internal/handler/landing/embed.go` | `//go:embed templates static` declarations exposing `Templates fs.FS` and `Static fs.FS` |
| `api/Dockerfile.landing` | Multi-stage build for the landing binary; copies templates+static via embed (so runtime needs only the binary) |

### Modified

| Path | Change |
|------|--------|
| `docker-compose.yml` | Add `landing` service with Traefik labels matching `Host(\`pages.localhost\`)` |
| `Makefile` | (No change — `up`/`down` from Plan C cover the new service automatically.) |

### Deleted
None.

---

## Tasks

### Task 1: Migration — `registrations` table + RLS

**Files:**
- Create: `api/internal/migrate/migrations/20260507000002_create_registrations.go`

- [ ] **Step 1: Write the migration**

```go
// api/internal/migrate/migrations/20260507000002_create_registrations.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE registrations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    child_name TEXT NOT NULL,
			    child_age INT,
			    parent_phone TEXT,
			    parent_email TEXT,
			    hifz_level TEXT,
			    status TEXT NOT NULL DEFAULT 'pending'
			      CHECK (status IN ('pending', 'approved', 'rejected')),
			    submitted_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_registrations_organization_id
			   ON registrations(organization_id)`,
			`CREATE INDEX idx_registrations_status
			   ON registrations(status)`,
			// app_role grants picked up automatically via the ALTER DEFAULT
			// PRIVILEGES set in 20260507000001 — but be explicit for clarity.
			`GRANT SELECT, INSERT, UPDATE, DELETE ON registrations TO mutqin_app`,
			`ALTER TABLE registrations ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE registrations FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON registrations
			   USING (organization_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)
			   WITH CHECK (organization_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP POLICY IF EXISTS tenant_isolation ON registrations`,
			`ALTER TABLE registrations NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE registrations DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE registrations`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/migrate/migrations/20260507000002_create_registrations.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "migrate: registrations table + RLS policy"
```

---

### Task 2: Bun model `Registration`

**Files:**
- Create: `api/internal/model/registration.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/registration.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Registration struct {
	bun.BaseModel `bun:"table:registrations,alias:r"`
	TenantScoped  // embed for per-model BeforeSelect/Update/Delete tenant filter

	ID             uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
	ChildName      string    `bun:"child_name,notnull"`
	ChildAge       *int      `bun:"child_age"`
	ParentPhone    *string   `bun:"parent_phone"`
	ParentEmail    *string   `bun:"parent_email"`
	HifzLevel      *string   `bun:"hifz_level"`
	Status         string    `bun:"status,notnull,nullzero,default:'pending'"`
	SubmittedAt    time.Time `bun:"submitted_at,notnull,nullzero,default:now()"`
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go build ./internal/model
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/model/registration.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "model: add Registration Bun model"
```

---

### Task 3: TDD `RegistrationRepo` — failing tests

**Files:**
- Create: `api/internal/repo/registration_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/repo/registration_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestRegistrationRepo_CreateUnderTenantCtx(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "Reg Org", Slug: "reg-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	tenantCtx := tenant.With(ctx, org.ID)
	regRepo := repo.NewRegistrationRepo(testAdmin) // admin handle so we don't need SET LOCAL for the test path

	r := &model.Registration{
		OrganizationID: org.ID,
		ChildName:      "Mahmoud",
	}
	if err := regRepo.Create(tenantCtx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID.String() == "00000000-0000-0000-0000-000000000000" {
		t.Fatal("expected ID populated after Create")
	}

	got, err := regRepo.List(tenantCtx, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].ChildName != "Mahmoud" {
		t.Fatalf("want child_name=Mahmoud, got %s", got[0].ChildName)
	}
}

func TestRegistrationRepo_TenantNarrowsList(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB)

	regRepo := repo.NewRegistrationRepo(testAdmin)
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgA.ID, ChildName: "A1"})
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgA.ID, ChildName: "A2"})
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgB.ID, ChildName: "B1"})

	// List under tenant=A: TenantScoped hook narrows.
	gotA, err := regRepo.List(tenant.With(ctx, orgA.ID), 100, 0)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("tenant A list: want 2, got %d", len(gotA))
	}

	// List under tenant=B: only B1.
	gotB, err := regRepo.List(tenant.With(ctx, orgB.ID), 100, 0)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(gotB) != 1 {
		t.Fatalf("tenant B list: want 1, got %d", len(gotB))
	}
}
```

- [ ] **Step 2: Run, expect compile failure (NewRegistrationRepo undefined)**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go test ./internal/repo/...
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/repo/registration_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "test: failing registration repo tests"
```

---

### Task 4: Implement `RegistrationRepo`

**Files:**
- Create: `api/internal/repo/registration.go`

- [ ] **Step 1: Write the repo**

```go
// api/internal/repo/registration.go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type RegistrationRepo struct {
	db bun.IDB
}

func NewRegistrationRepo(db bun.IDB) *RegistrationRepo {
	return &RegistrationRepo{db: db}
}

func (r *RegistrationRepo) Create(ctx context.Context, reg *model.Registration) error {
	_, err := r.db.NewInsert().Model(reg).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert registration: %w", err)
	}
	return nil
}

func (r *RegistrationRepo) List(ctx context.Context, limit, offset int) ([]model.Registration, error) {
	var regs []model.Registration
	err := r.db.NewSelect().
		Model(&regs).
		OrderExpr("submitted_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list registrations: %w", err)
	}
	return regs, nil
}
```

- [ ] **Step 2: Run repo tests, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go test -run TestRegistration ./internal/repo/... -v 2>&1 | tail -10
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/repo/registration.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "repo: implement RegistrationRepo with Bun"
```

---

### Task 5: Add HTML templates

**Files:**
- Create: `api/internal/handler/landing/templates/layout.html.tmpl`
- Create: `api/internal/handler/landing/templates/center.html.tmpl`
- Create: `api/internal/handler/landing/templates/register.html.tmpl`
- Create: `api/internal/handler/landing/templates/success.html.tmpl`
- Create: `api/internal/handler/landing/templates/not_found.html.tmpl`
- Create: `api/internal/handler/landing/static/style.css`

These five templates use a shared layout via `{{define}}` / `{{template}}`. They're rendered server-side; no JavaScript.

- [ ] **Step 1: Write `layout.html.tmpl`**

```html
{{- /* api/internal/handler/landing/templates/layout.html.tmpl */ -}}
<!doctype html>
<html lang="ar" dir="rtl">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ block "title" . }}Mutqin{{ end }}</title>
  <link rel="stylesheet" href="/static/style.css">
</head>
<body>
  <main class="container">
    {{ block "content" . }}{{ end }}
  </main>
</body>
</html>
```

- [ ] **Step 2: Write `center.html.tmpl`**

```html
{{- /* api/internal/handler/landing/templates/center.html.tmpl */ -}}
{{ define "title" }}{{ .Org.Name }} — Mutqin{{ end }}

{{ define "content" }}
<h1>{{ .Org.Name }}</h1>
{{ if .Org.Description }}
  <p class="description">{{ .Org.Description }}</p>
{{ end }}
{{ if .Org.City.Valid }}
  <p class="meta">{{ .Org.City.String }}</p>
{{ end }}
<a class="cta" href="/{{ .Org.Slug }}/register">سجّل ابنك / Register your child</a>
{{ end }}
```

(Adapt template helpers if `Org.City` is `*string` rather than a `sql.NullString`-style struct — see Step 6 where the handler shapes the data.)

- [ ] **Step 3: Write `register.html.tmpl`**

```html
{{- /* api/internal/handler/landing/templates/register.html.tmpl */ -}}
{{ define "title" }}Register — {{ .Org.Name }}{{ end }}

{{ define "content" }}
<h1>Register your child at {{ .Org.Name }}</h1>
<form method="POST" action="/{{ .Org.Slug }}/register">
  <label>
    Child's name (required)
    <input type="text" name="child_name" required maxlength="120">
  </label>
  <label>
    Child's age
    <input type="number" name="child_age" min="3" max="25">
  </label>
  <label>
    Parent's phone
    <input type="tel" name="parent_phone" inputmode="tel" maxlength="32">
  </label>
  <label>
    Parent's email
    <input type="email" name="parent_email" maxlength="120">
  </label>
  <label>
    Hifz level
    <select name="hifz_level">
      <option value="">Not started</option>
      <option value="some_juz">Some juz memorized</option>
      <option value="multiple_juz">Multiple juz memorized</option>
      <option value="hafiz">Completed hifz</option>
    </select>
  </label>
  {{ if .Error }}<p class="error">{{ .Error }}</p>{{ end }}
  <button type="submit" class="cta">Submit</button>
</form>
{{ end }}
```

- [ ] **Step 4: Write `success.html.tmpl`**

```html
{{- /* api/internal/handler/landing/templates/success.html.tmpl */ -}}
{{ define "title" }}Submitted — {{ .Org.Name }}{{ end }}

{{ define "content" }}
<h1>Thank you</h1>
<p>{{ .Org.Name }} has received the registration for <strong>{{ .ChildName }}</strong>. The center will contact you to confirm.</p>
<a class="cta" href="/{{ .Org.Slug }}">Back to {{ .Org.Name }}</a>
{{ end }}
```

- [ ] **Step 5: Write `not_found.html.tmpl`**

```html
{{- /* api/internal/handler/landing/templates/not_found.html.tmpl */ -}}
{{ define "title" }}Not found{{ end }}

{{ define "content" }}
<h1>Center not found</h1>
<p>The page you're looking for doesn't exist.</p>
{{ end }}
```

- [ ] **Step 6: Write `static/style.css`**

```css
/* api/internal/handler/landing/static/style.css */
:root {
  --color-primary: #059669;
  --color-text: #111827;
  --color-muted: #6b7280;
  --color-bg: #f9fafb;
  --color-border: #e5e7eb;
  --radius: 8px;
}

* { box-sizing: border-box; }
body {
  margin: 0;
  font: 16px/1.5 system-ui, -apple-system, "Cairo", sans-serif;
  color: var(--color-text);
  background: var(--color-bg);
}
.container {
  max-width: 640px;
  margin: 0 auto;
  padding: 1.5rem 1rem;
}
h1 { font-size: 1.75rem; margin: 0 0 1rem; }
p { margin: 0 0 0.75rem; }
p.description { color: var(--color-text); }
p.meta { color: var(--color-muted); font-size: 0.95rem; }
p.error { color: #dc2626; background: #fee2e2; padding: 0.5rem 0.75rem; border-radius: var(--radius); }
form label { display: block; margin: 0 0 1rem; }
form input, form select {
  display: block;
  width: 100%;
  margin-top: 0.25rem;
  padding: 0.5rem 0.75rem;
  font: inherit;
  border: 1px solid var(--color-border);
  border-radius: var(--radius);
  background: #fff;
}
form input:focus, form select:focus {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
  border-color: var(--color-primary);
}
.cta {
  display: inline-block;
  background: var(--color-primary);
  color: #fff;
  text-decoration: none;
  padding: 0.75rem 1.25rem;
  border-radius: var(--radius);
  border: 0;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}
.cta:hover { filter: brightness(0.95); }
```

- [ ] **Step 7: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/handler/landing/templates api/internal/handler/landing/static
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "landing: HTML templates + minimal CSS"
```

---

### Task 6: Embedded FS + handler

**Files:**
- Create: `api/internal/handler/landing/embed.go`
- Create: `api/internal/handler/landing/handler.go`

- [ ] **Step 1: Write the embed file**

```go
// api/internal/handler/landing/embed.go
package landing

import (
	"embed"
	"io/fs"
)

//go:embed templates
var templatesDir embed.FS

//go:embed static
var staticDir embed.FS

// Templates returns a sub-FS rooted at the templates directory.
func Templates() fs.FS {
	sub, err := fs.Sub(templatesDir, "templates")
	if err != nil {
		panic(err) // package-level invariant — fails at init if the embed is misconfigured.
	}
	return sub
}

// Static returns a sub-FS rooted at the static directory (CSS, etc.).
func Static() fs.FS {
	sub, err := fs.Sub(staticDir, "static")
	if err != nil {
		panic(err)
	}
	return sub
}
```

- [ ] **Step 2: Write the handler**

```go
// api/internal/handler/landing/handler.go
package landing

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// OrgLookup resolves a slug to (organization, true) or (_, false) if unknown.
type OrgLookup interface {
	GetBySlugAdmin(ctx context.Context, slug string) (*model.Organization, error)
}

// Handler bundles the dependencies for landing routes.
type Handler struct {
	orgs       OrgLookup
	regs       *repo.RegistrationRepo
	tmpl       *template.Template
	staticFS   fs.FS
	logger     *slog.Logger
}

// New constructs a Handler. Templates are parsed once at construction.
func New(orgs OrgLookup, regs *repo.RegistrationRepo, logger *slog.Logger) (*Handler, error) {
	tmpls, err := template.ParseFS(Templates(),
		"layout.html.tmpl",
		"center.html.tmpl",
		"register.html.tmpl",
		"success.html.tmpl",
		"not_found.html.tmpl",
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Handler{
		orgs:     orgs,
		regs:     regs,
		tmpl:     tmpls,
		staticFS: Static(),
		logger:   logger,
	}, nil
}

// Routes returns a Chi mux mounted at the root of the landing domain.
func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/static/*", h.serveStatic)
	r.Get("/{slug}", h.center)
	r.Get("/{slug}/register", h.registerForm)
	r.Post("/{slug}/register", h.submitRegistration)
	r.NotFound(h.notFound)
	return r
}

func (h *Handler) serveStatic(w http.ResponseWriter, r *http.Request) {
	// Strip the /static/ prefix and serve from the embedded FS.
	stripped := strings.TrimPrefix(r.URL.Path, "/static/")
	http.ServeFileFS(w, r, h.staticFS, stripped)
}

func (h *Handler) resolveOrg(ctx context.Context, slug string) (*model.Organization, context.Context, bool) {
	org, err := h.orgs.GetBySlugAdmin(ctx, slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, ctx, false
		}
		h.logger.Error("org lookup", "slug", slug, "error", err)
		return nil, ctx, false
	}
	return org, tenant.With(ctx, org.ID), true
}

func (h *Handler) center(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, _, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}
	h.render(w, "layout.html.tmpl", map[string]any{
		"Org": org,
	})
}

func (h *Handler) registerForm(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, _, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}
	// Reuse the layout but render the "register" content block. We achieve
	// this by parsing both layout and register templates together (already
	// done in New) and executing layout with the data — register's blocks
	// override the layout's defaults.
	h.tmpl.Lookup("register.html.tmpl").Execute(w, map[string]any{"Org": org})
	_ = h.tmpl.Lookup("layout.html.tmpl").Execute(w, map[string]any{"Org": org})
}

func (h *Handler) submitRegistration(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	org, ctx, ok := h.resolveOrg(r.Context(), slug)
	if !ok {
		h.notFound(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	childName := strings.TrimSpace(r.PostForm.Get("child_name"))
	if childName == "" {
		// Re-render the form with an error.
		_ = h.tmpl.ExecuteTemplate(w, "layout.html.tmpl", map[string]any{
			"Org":   org,
			"Error": "Child's name is required.",
		})
		return
	}

	reg := &model.Registration{
		OrganizationID: org.ID,
		ChildName:      childName,
	}
	if v := strings.TrimSpace(r.PostForm.Get("child_age")); v != "" {
		if age, err := strconv.Atoi(v); err == nil && age > 0 && age < 100 {
			reg.ChildAge = &age
		}
	}
	if v := strings.TrimSpace(r.PostForm.Get("parent_phone")); v != "" {
		reg.ParentPhone = &v
	}
	if v := strings.TrimSpace(r.PostForm.Get("parent_email")); v != "" {
		reg.ParentEmail = &v
	}
	if v := strings.TrimSpace(r.PostForm.Get("hifz_level")); v != "" {
		reg.HifzLevel = &v
	}

	if err := h.regs.Create(ctx, reg); err != nil {
		h.logger.Error("registration create", "slug", slug, "error", err)
		http.Error(w, "could not save registration", http.StatusInternalServerError)
		return
	}

	_ = h.tmpl.ExecuteTemplate(w, "layout.html.tmpl", map[string]any{
		"Org":       org,
		"ChildName": childName,
	})
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	_ = h.tmpl.ExecuteTemplate(w, "layout.html.tmpl", map[string]any{})
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("render template", "template", name, "error", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// Compile-time guard: uuid.Nil is the zero value used to detect "not set" by callers.
var _ = uuid.Nil
```

> **Implementer note:** Go `html/template` blocks behave such that executing the layout will render whatever `{{ block "content" . }}` finds in the *parsed-together* template set. The handler renders the layout and the per-page templates side-by-side because they share `define` blocks. If during testing a particular page's content doesn't appear, switch the call to `ExecuteTemplate(w, "<page>.html.tmpl", data)` — Go's templates will resolve the layout's `block` from the page's own `define` because they're in the same set.
> The handler above renders by executing the layout template, which is the standard pattern.

- [ ] **Step 3: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go build ./internal/handler/landing
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/internal/handler/landing/embed.go api/internal/handler/landing/handler.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "landing: HTTP handlers + embedded templates"
```

---

### Task 7: `cmd/landing/main.go` — wire the binary

**Files:**
- Create: `api/cmd/landing/main.go`

- [ ] **Step 1: Write main**

```go
// api/cmd/landing/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler/landing"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/repo"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Admin handle for cross-tenant slug lookup.
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	// App handle for INSERTs into RLS-subject tables (registrations).
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false)
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()

	orgRepo := repo.NewOrganizationRepo(adminDB)
	regRepo := repo.NewRegistrationRepo(appDB)

	handler, err := landing.New(orgRepo, regRepo, logger)
	if err != nil {
		slog.Error("init landing handler", "error", err)
		os.Exit(1)
	}

	// Compose middleware: request_id → logger → RLSContext → routes.
	// Tenant ctx is set INSIDE the handler (slug from path, not Host), so the
	// resolver middleware does not apply here. RLSContext picks up tenant from
	// ctx and sets app.current_tenant for the duration of the request — it
	// no-ops for routes that don't carry a tenant (like /static/*).
	mux := http.NewServeMux()
	mux.Handle("/", middleware.RequestID(
		middleware.Logger(logger)(
			middleware.RLSContext(middleware.NewBunRunner(appDB))(
				handler.Routes(),
			),
		),
	))

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("landing server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("landing server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("landing server shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("landing server shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("landing server stopped")
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/cmd/landing/main.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "cmd: landing binary entry"
```

---

### Task 8: `api/Dockerfile.landing`

**Files:**
- Create: `api/Dockerfile.landing`

- [ ] **Step 1: Write the Dockerfile**

```dockerfile
# api/Dockerfile.landing
# syntax=docker/dockerfile:1.7

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/landing ./cmd/landing

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/landing /app/landing
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/landing"]
```

- [ ] **Step 2: Build the image**

```bash
docker build -t mutqin-landing:dev -f /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api/Dockerfile.landing /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api 2>&1 | tail -5
```

Expected: clean build. Image size ~12-15 MB (templates + CSS embedded).

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add api/Dockerfile.landing
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "build: landing Dockerfile (multi-stage distroless)"
```

---

### Task 9: Add `landing` service to compose

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Append the landing service inside `services:` block (place between `web` and `traefik`)**

The full updated services block:

```yaml
  # ... web service unchanged ...

  landing:
    build:
      context: ./api
      dockerfile: Dockerfile.landing
    image: mutqin-landing:dev
    container_name: mutqin-landing
    environment:
      DATABASE_URL: postgres://mutqin:mutqin@db:5432/mutqin?sslmode=disable
      APP_DATABASE_URL: postgres://mutqin_app:mutqin_app@db:5432/mutqin?sslmode=disable
      JWT_SECRET: dev-secret    # not used by landing but required by config.Load
      BASE_HOST: localhost
      PORT: "8080"
    depends_on:
      db:
        condition: service_healthy
    labels:
      - traefik.enable=true
      - traefik.docker.network=mutqin
      - traefik.http.routers.landing.rule=Host(`pages.localhost`)
      - traefik.http.routers.landing.entrypoints=web
      - traefik.http.routers.landing.priority=50
      - traefik.http.services.landing.loadbalancer.server.port=8080
    networks:
      - mutqin

  # ... traefik service unchanged, but add landing to depends_on ...
```

The full new compose: keep db, redis, api, web exactly as in Plan D, insert landing after web, and add `landing` to traefik's `depends_on` list:

```yaml
  traefik:
    image: traefik:v3.1
    container_name: mutqin-traefik
    command:
      - --configFile=/etc/traefik/traefik.yml
    ports:
      - "80:80"
      - "8081:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./infra/traefik/traefik.yml:/etc/traefik/traefik.yml:ro
    depends_on:
      - api
      - web
      - landing
    networks:
      - mutqin
```

- [ ] **Step 2: Bring the stack up**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e
docker compose up -d --build
sleep 18
docker compose ps
```

Expected: 6 containers — db, redis, api, web, landing, traefik.

- [ ] **Step 3: Smoke matrix — landing routes**

```bash
# Set up test data via api: create one organization with slug "noor".
docker exec -i mutqin-db psql -U mutqin -d mutqin <<'SQL'
INSERT INTO organizations (name, slug, country, tier, status)
VALUES ('Markaz An-Noor', 'noor', 'SO', 'free', 'active')
ON CONFLICT (slug) DO NOTHING;
SQL

# Center landing
echo "--- pages.localhost/noor ---"
curl -s -i -H "Host: pages.localhost" http://localhost/noor | head -8

# Registration form
echo "--- pages.localhost/noor/register ---"
curl -s -i -H "Host: pages.localhost" http://localhost/noor/register | head -5

# Submit a registration
echo "--- POST registration ---"
curl -s -i -H "Host: pages.localhost" \
  -d "child_name=Mahmoud&child_age=8&parent_phone=+25261...&hifz_level=some_juz" \
  http://localhost/noor/register | head -5

# Confirm row landed in DB (admin handle, bypasses RLS)
echo "--- DB check ---"
docker exec -i mutqin-db psql -U mutqin -d mutqin -c "SELECT child_name, child_age, hifz_level, status FROM registrations;"

# Unknown slug → 404
echo "--- pages.localhost/ghost ---"
curl -s -i -H "Host: pages.localhost" -o /dev/null -w "%{http_code}\n" http://localhost/ghost
```

Expected:
- `/noor` → 200, HTML containing `Markaz An-Noor`
- `/noor/register` → 200, form fields visible
- POST → 200, success page with "Mahmoud"
- DB row: `Mahmoud, 8, some_juz, pending`
- `/ghost` → 404

- [ ] **Step 4: Tear down**

```bash
docker compose down
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e add docker-compose.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e commit -m "infra: add landing service (Host: pages.localhost)"
```

---

### Task 10: Final verification + push

- [ ] **Step 1: Run full test suite (host-mode Go)**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e/api && go clean -testcache && go test ./... 2>&1 | tail -10
```

Expected: all PASS, including new `TestRegistration*` tests in repo package.

- [ ] **Step 2: vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Push**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e push -u origin feat/plan-e-landing-container
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-e push gitlab feat/plan-e-landing-container
```

- [ ] **Step 4: Open PR + MR**

Base: `docs/backend-architecture-v2`.

Title: `Plan E — Landing container (Go HTML templates)`

Body:

```
## Summary
- New `landing` Go binary serving server-rendered HTML for parent-facing pages at `pages.localhost` (production: `pages.mutqin.app`).
- Templates and CSS embedded via `embed.FS` — single-file distroless image (~12 MB).
- New `registrations` table + RLS policy + `RegistrationRepo` (Bun, embeds TenantScoped).
- Routes: `GET /{slug}`, `GET /{slug}/register`, `POST /{slug}/register`. Slug → org via `GetBySlugAdmin`; tenant ctx + RLS context populated per-request.
- Compose grows to 6 services; Traefik routes `Host(pages.localhost)` → landing at priority 50.

## Verification
- [x] `make up` brings up 6 containers; db `healthy`.
- [x] `pages.localhost/noor` → 200 HTML with center name.
- [x] `pages.localhost/noor/register` → 200 form.
- [x] POST → 200 success; `registrations` row exists with status=pending.
- [x] `pages.localhost/ghost` → 404.

## Out of scope (deferred)
- Tenant theming on the landing page (separate Spec 2).
- Email/SMS notification on new registration (messaging epic).
- GitLab CI building the landing image (Plan F).
```

---

## Self-Review

**Spec coverage:**
- D4 (`landing` is its own container, Go templates) — Tasks 5–9 ✓
- Spec D-table row "pages.mutqin.app" — Task 9 routes via Host(pages.localhost) locally; same pattern for prod with BASE_HOST/Cloudflare records.
- Spec API table row "Public Landing Pages" (`/public/:slug`, `/public/:slug/announcements`, `/public/:slug/register`) — covered partially: this plan handles slug + register; announcements deferred (no announcements table yet).
- Required-changes #5 (split landing handlers into `internal/handler/landing/`) — Tasks 5–7 ✓
- Schema for `registrations` table from architecture.md — Task 1 ✓

**Placeholder scan:** No "TBD"/"TODO" / "fill in details" / "similar to". Every step has the actual content. The implementer note in Task 6 calls out a Go-template gotcha but does not defer functionality.

**Type / signature consistency:**
- `repo.NewRegistrationRepo(bun.IDB)` consistent across Tasks 3, 4, 7 (test, impl, main).
- `landing.New(orgs landing.OrgLookup, regs *repo.RegistrationRepo, logger *slog.Logger)` consistent in Tasks 6 and 7.
- `landing.OrgLookup` interface satisfied by `*repo.OrganizationRepo.GetBySlugAdmin` (signature `(ctx, slug) (*model.Organization, error)`).
- `model.Registration` field names + types match Bun tags + migration column names + form field names (`child_name`, `child_age`, `parent_phone`, `parent_email`, `hifz_level`).
- Compose service name `landing`, image `mutqin-landing:dev`, internal port `8080` consistent throughout.

**Scope:** 10 tasks. Substantive: 1, 2, 3, 4, 6, 7, 9. Trivial: 5 (templates), 8 (Dockerfile is short), 10 (verify).
