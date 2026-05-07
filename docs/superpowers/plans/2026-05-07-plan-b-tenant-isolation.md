# Plan B — Tenant Isolation Defense-in-Depth

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce tenant isolation on the existing tenant-scoped tables (`users`, `invites`) through three independent layers — Bun query hook, Postgres RLS, per-request `SET LOCAL` — so that any single failure is caught by the other two.

**Architecture:** A `tenant` package owns the `context.Context` carrier. A Bun `BeforeQuery` hook reads tenant ID from ctx and rewrites every `Select`/`Update`/`Delete` to add `WHERE organization_id = ?` automatically. The application connects as a non-superuser `app_role` so Postgres-level RLS policies actually fire. HTTP middleware parses the tenant from `Host`/`X-Tenant-Slug`, looks the slug up to a UUID, and sets both the ctx value and `SET LOCAL app.current_tenant` on the connection.

**Tech Stack:**
- Existing: Bun ORM, Postgres 16, Chi, slog, testcontainers-go
- New types: `tenant.Context`, `db.TenantHook`, `middleware.TenantResolver`, `middleware.RLSContext`
- Postgres features: `CREATE ROLE`, `ENABLE ROW LEVEL SECURITY`, `FORCE ROW LEVEL SECURITY`, `CREATE POLICY`, `current_setting('app.current_tenant', true)`

**Out of scope (deferred to later plans):**
- JWT-based tenant resolution (Plan adds Epic 1.2 OTP/JWT)
- Redis caching of slug→UUID lookups (in-memory map suffices at 50-tenant target)
- RLS on tables that don't exist yet (`halaqat`, `students`, etc. — added by their own epic plans)
- `cmd/landing` binary's tenant resolver (Plan E adds the landing container)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/tenant/context.go` | ctx carrier — `From(ctx) (uuid.UUID, bool)`, `With(ctx, uuid) ctx` |
| `api/internal/tenant/context_test.go` | Round-trip ctx helper tests |
| `api/internal/db/hooks.go` | `TenantHook` implementing `bun.QueryHook` — rewrites `SelectQuery`/`UpdateQuery`/`DeleteQuery` |
| `api/internal/db/hooks_test.go` | Unit tests asserting the rewritten SQL contains `organization_id = ?` |
| `api/internal/migrate/migrations/20260507000001_app_role_and_rls.go` | Creates `app_role`, GRANTs DML on existing tables, ENABLE+FORCE RLS, policies on `users` + `invites` |
| `api/internal/middleware/request_id.go` | Generates / propagates `X-Request-ID` |
| `api/internal/middleware/logger.go` | Structured `slog` access log including `request_id`, `org_id` |
| `api/internal/middleware/tenant.go` | Resolves tenant from `Host` (`{slug}.mutqin.app`) or `X-Tenant-Slug` header → looks up org → injects into ctx via `tenant.With` |
| `api/internal/middleware/tenant_test.go` | HTTP unit tests for the resolver |
| `api/internal/middleware/rls.go` | Wraps a request in a transaction whose first statement is `SET LOCAL app.current_tenant` |
| `api/internal/middleware/rls_test.go` | Unit test asserting `SET LOCAL` actually fires |
| `api/internal/repo/lookup.go` | `OrganizationRepo.GetBySlugAdmin` — bypass-tenant lookup for the resolver chain (uses superuser conn) |

### Modified

| Path | Change |
|------|--------|
| `api/internal/config/config.go` | Add `AppDatabaseURL` env var (non-superuser DSN) alongside existing `DatabaseURL` |
| `api/internal/db/conn.go` | `NewDB` accepts an optional list of `bun.QueryHook` so callers can install `TenantHook` at construction |
| `api/internal/repo/repo_test.go` | Bootstrap connection runs migrations as superuser; `testDB` exposes the **app_role** connection with `TenantHook` installed |
| `api/cmd/server/main.go` | Open two `bun.DB` handles — superuser for migrations, app_role for serving — install `TenantHook` on the app handle, wire new middleware chain |
| `Makefile` | Add `APP_DATABASE_URL` env var |

### Deleted
None.

---

## Tasks

### Task 1: Add `tenant` context package

**Files:**
- Create: `api/internal/tenant/context.go`
- Create: `api/internal/tenant/context_test.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/tenant/context_test.go
package tenant_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestWithAndFrom_RoundTrip(t *testing.T) {
	id := uuid.New()
	ctx := tenant.With(context.Background(), id)

	got, ok := tenant.From(ctx)
	if !ok {
		t.Fatal("From returned ok=false on a populated ctx")
	}
	if got != id {
		t.Fatalf("got %s, want %s", got, id)
	}
}

func TestFrom_EmptyCtx(t *testing.T) {
	_, ok := tenant.From(context.Background())
	if ok {
		t.Fatal("From returned ok=true on an empty ctx")
	}
}
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/tenant/...
```

Expected: `package internal/tenant: no Go files`.

- [ ] **Step 3: Implement**

```go
// api/internal/tenant/context.go
package tenant

import (
	"context"

	"github.com/google/uuid"
)

type ctxKey struct{}

// With returns a child ctx carrying the resolved tenant organization ID.
func With(ctx context.Context, orgID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKey{}, orgID)
}

// From returns the tenant organization ID from ctx and ok=true if present.
func From(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(ctxKey{}).(uuid.UUID)
	return v, ok
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/tenant/...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/tenant/
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "tenant: add ctx carrier"
```

---

### Task 2: Add Bun `TenantHook`

**Files:**
- Create: `api/internal/db/hooks.go`
- Create: `api/internal/db/hooks_test.go`

- [ ] **Step 1: Write the failing test**

```go
// api/internal/db/hooks_test.go
package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/schema"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

type tenantTestRow struct {
	bun.BaseModel  `bun:"table:tenant_test_rows,alias:t"`
	ID             uuid.UUID `bun:",pk,type:uuid"`
	OrganizationID uuid.UUID `bun:"organization_id,type:uuid,notnull"`
}

// rendererBunDB constructs a Bun DB with the postgres dialect but no real *sql.DB,
// purely so we can assemble a query and call AppendQuery to inspect the generated SQL.
func rendererBunDB() *bun.DB {
	return bun.NewDB(nil, pgdialect.New())
}

func renderSelectSQL(ctx context.Context, hook db.TenantHook) string {
	bdb := rendererBunDB()
	bdb.AddQueryHook(hook)
	q := bdb.NewSelect().Model((*tenantTestRow)(nil))
	// AppendQuery triggers BeforeQuery via the hook chain.
	bytes, _ := q.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	_ = ctx
	return string(bytes)
}

func TestTenantHook_InjectsWhere_OnSelect(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	bdb := rendererBunDB()
	bdb.AddQueryHook(db.TenantHook{})

	q := bdb.NewSelect().Model((*tenantTestRow)(nil))
	// Trigger BeforeQuery by serializing the query in the hook-aware path.
	_, err := q.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	if err != nil {
		// Ignore — we want SQL string only.
	}
	// Re-run with ctx so the hook sees it.
	hook := db.TenantHook{}
	q2 := bdb.NewSelect().Model((*tenantTestRow)(nil))
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q2})
	sql, _ := q2.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	got := string(sql)
	if !strings.Contains(got, `"organization_id" =`) {
		t.Fatalf("SELECT did not get organization_id filter, sql=%q", got)
	}
}

func TestTenantHook_NoCtx_NoFilter(t *testing.T) {
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewSelect().Model((*tenantTestRow)(nil))
	hook.BeforeQuery(context.Background(), &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	if strings.Contains(string(sql), "organization_id") {
		t.Fatalf("expected no organization_id filter on empty ctx, got %q", string(sql))
	}
}

func TestTenantHook_Update(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewUpdate().Model(&tenantTestRow{ID: uuid.New()}).Set("organization_id = ?", uuid.New()).Where("id = ?", uuid.New())
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	if !strings.Contains(string(sql), `"organization_id" =`) {
		t.Fatalf("UPDATE did not get organization_id filter, sql=%q", string(sql))
	}
}

func TestTenantHook_Delete(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewDelete().Model((*tenantTestRow)(nil)).Where("id = ?", uuid.New())
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewFormatter(pgdialect.New()), nil)
	if !strings.Contains(string(sql), `"organization_id" =`) {
		t.Fatalf("DELETE did not get organization_id filter, sql=%q", string(sql))
	}
}
```

- [ ] **Step 2: Run, expect failure**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/db/...
```

Expected: `undefined: db.TenantHook`.

- [ ] **Step 3: Implement**

```go
// api/internal/db/hooks.go
package db

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// TenantHook auto-injects "WHERE organization_id = ?" on Select / Update / Delete
// queries whenever the request context carries a resolved tenant.
//
// Repo code MUST NOT add organization_id filters manually — this hook owns that
// concern. Bypass is achieved by NOT placing a tenant in the ctx (e.g., for
// admin/setup paths that intentionally cross tenants).
type TenantHook struct{}

var _ bun.QueryHook = TenantHook{}

func (TenantHook) BeforeQuery(ctx context.Context, e *bun.QueryEvent) context.Context {
	orgID, ok := tenant.From(ctx)
	if !ok {
		return ctx
	}
	switch q := e.IQuery.(type) {
	case *bun.SelectQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.UpdateQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.DeleteQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return ctx
}

func (TenantHook) AfterQuery(ctx context.Context, e *bun.QueryEvent) {}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/db/...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/db/hooks.go api/internal/db/hooks_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "db: add TenantHook auto-injecting WHERE organization_id"
```

---

### Task 3: Let `db.NewDB` accept query hooks

**Files:**
- Modify: `api/internal/db/conn.go`

- [ ] **Step 1: Replace `NewDB` to accept variadic hooks**

```go
// api/internal/db/conn.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"
)

// NewDB opens a Bun DB backed by pgdriver, verifies connectivity, installs the
// supplied query hooks, and returns the handle. The caller is responsible for
// closing it.
//
// debug=true installs bundebug to log every query — only enable in development.
func NewDB(ctx context.Context, databaseURL string, debug bool, hooks ...bun.QueryHook) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(30 * time.Minute)

	bdb := bun.NewDB(sqldb, pgdialect.New())
	for _, h := range hooks {
		bdb.AddQueryHook(h)
	}
	if debug {
		bdb.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := bdb.PingContext(pingCtx); err != nil {
		_ = bdb.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return bdb, nil
}
```

- [ ] **Step 2: Verify build (existing callers pass no hooks; signature is back-compat)**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go build ./... && go test ./internal/repo/... 2>&1 | tail -3
```

Expected: clean build; existing repo tests still pass (they don't install the hook yet, so behavior unchanged).

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/db/conn.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "db: NewDB accepts variadic query hooks"
```

---

### Task 4: Migration — create `app_role`, GRANT, enable RLS, policies

**Files:**
- Create: `api/internal/migrate/migrations/20260507000001_app_role_and_rls.go`

- [ ] **Step 1: Write the migration**

```go
// api/internal/migrate/migrations/20260507000001_app_role_and_rls.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			// Idempotent role creation. NOLOGIN until the migration explicitly grants login.
			`DO $$
			 BEGIN
			   IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'mutqin_app') THEN
			     CREATE ROLE mutqin_app LOGIN PASSWORD 'mutqin_app';
			   END IF;
			 END $$`,
			// Schema usage + DML on existing tables.
			`GRANT USAGE ON SCHEMA public TO mutqin_app`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO mutqin_app`,
			`GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO mutqin_app`,
			// Future tables get the same grants automatically.
			`ALTER DEFAULT PRIVILEGES IN SCHEMA public
			   GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO mutqin_app`,
			`ALTER DEFAULT PRIVILEGES IN SCHEMA public
			   GRANT USAGE, SELECT ON SEQUENCES TO mutqin_app`,
			// Enable + FORCE RLS so even table owners are subject (the role we run
			// as today, mutqin, owns the tables and would otherwise bypass policies).
			`ALTER TABLE users ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE users FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE invites ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE invites FORCE ROW LEVEL SECURITY`,
			// Policy: only see rows whose organization_id matches the per-request
			// app.current_tenant GUC. No tenant set => no rows visible.
			`CREATE POLICY tenant_isolation ON users
			   USING (organization_id = current_setting('app.current_tenant', true)::uuid)
			   WITH CHECK (organization_id = current_setting('app.current_tenant', true)::uuid)`,
			`CREATE POLICY tenant_isolation ON invites
			   USING (organization_id = current_setting('app.current_tenant', true)::uuid)
			   WITH CHECK (organization_id = current_setting('app.current_tenant', true)::uuid)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP POLICY IF EXISTS tenant_isolation ON invites`,
			`DROP POLICY IF EXISTS tenant_isolation ON users`,
			`ALTER TABLE invites NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE invites DISABLE ROW LEVEL SECURITY`,
			`ALTER TABLE users NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE users DISABLE ROW LEVEL SECURITY`,
			`REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM mutqin_app`,
			`REVOKE ALL ON ALL TABLES IN SCHEMA public FROM mutqin_app`,
			`REVOKE USAGE ON SCHEMA public FROM mutqin_app`,
			`DROP ROLE IF EXISTS mutqin_app`,
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
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/migrate/migrations/20260507000001_app_role_and_rls.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "migrate: app_role + RLS policies on users/invites"
```

---

### Task 5: Add `OrganizationRepo.GetBySlugAdmin` (cross-tenant lookup)

**Rationale:** The tenant resolver needs to look up an org by slug *before* it knows which tenant it is. Looking up via the regular `GetBySlug` would not work once `users`/`invites` are RLS'd, but `organizations` itself isn't tenant-scoped. We just need an explicit name to make it clear this is a deliberate admin-path query, not a bug.

**Files:**
- Modify: `api/internal/repo/organization.go` — add new method

- [ ] **Step 1: Append the new method**

Open `api/internal/repo/organization.go`, append at the bottom (just before the closing brace of the file is the last method `List`; add this immediately after `List`):

```go
// GetBySlugAdmin looks up an organization by slug WITHOUT a tenant in ctx.
// This is the entry point for the tenant-resolution chain: the request arrives,
// we extract the slug from Host or X-Tenant-Slug, and we need the UUID before
// we can populate the tenant ctx. Equivalent in behavior to GetBySlug; the
// distinct name flags it as an intentional cross-tenant call site.
func (r *OrganizationRepo) GetBySlugAdmin(ctx context.Context, slug string) (*model.Organization, error) {
	return r.GetBySlug(ctx, slug)
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go build ./internal/repo
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/repo/organization.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "repo: add GetBySlugAdmin alias for cross-tenant resolver lookup"
```

---

### Task 6: Update test fixture — dual connection (superuser + app_role)

**Files:**
- Modify: `api/internal/repo/repo_test.go`

The fixture now: connects as superuser to run migrations, then opens a second `bun.DB` as `mutqin_app` and exposes that as `testDB`. The app connection has the `TenantHook` installed so repo tests automatically exercise the hook.

- [ ] **Step 1: Replace the file contents**

```go
// api/internal/repo/repo_test.go
package repo_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)

var (
	testDB     *bun.DB // mutqin_app connection — RLS-subject, TenantHook installed
	testAdmin  *bun.DB // mutqin superuser — no RLS, no hook (used only by truncateAll and admin lookups)
)

// truncateAll empties every tenant-bearing table. Tests should call it via
// t.Cleanup so each test sees a known-empty database. Uses the admin handle so
// it bypasses RLS.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testAdmin.ExecContext(context.Background(),
		`TRUNCATE TABLE otp_codes, invites, users, organizations RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgC, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("mutqin_test"),
		tcpostgres.WithUsername("mutqin"),
		tcpostgres.WithPassword("mutqin"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		log.Fatalf("start postgres testcontainer: %v", err)
	}
	defer func() {
		if err := pgC.Terminate(ctx); err != nil {
			log.Printf("terminate testcontainer: %v", err)
		}
	}()

	host, err := pgC.Host(ctx)
	if err != nil {
		log.Fatalf("container host: %v", err)
	}
	port, err := pgC.MappedPort(ctx, "5432")
	if err != nil {
		log.Fatalf("container port: %v", err)
	}

	adminDSN := fmt.Sprintf("postgres://mutqin:mutqin@%s:%s/mutqin_test?sslmode=disable", host, port.Port())
	appDSN := fmt.Sprintf("postgres://mutqin_app:mutqin_app@%s:%s/mutqin_test?sslmode=disable", host, port.Port())

	admin, err := db.NewDB(ctx, adminDSN, false)
	if err != nil {
		log.Fatalf("connect admin: %v", err)
	}
	defer admin.Close()
	testAdmin = admin

	if err := migrate.Up(ctx, admin); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	app, err := db.NewDB(ctx, appDSN, false, db.TenantHook{})
	if err != nil {
		log.Fatalf("connect app: %v", err)
	}
	defer app.Close()
	testDB = app

	os.Exit(m.Run())
}
```

- [ ] **Step 2: Run all repo tests, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/repo/... 2>&1 | tail -10
```

Expected: All 7 existing tests still PASS (they explicitly set the org/user fields, don't rely on RLS behavior). The hook is installed, but tests don't yet put a tenant in ctx, so the hook is a no-op for them.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/repo/repo_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "test(repo): dual-connection fixture (admin + app_role with TenantHook)"
```

---

### Task 7: Repo test — TenantHook narrows results to tenant

**Files:**
- Create: `api/internal/repo/tenant_hook_integration_test.go`

- [ ] **Step 1: Write the test**

```go
// api/internal/repo/tenant_hook_integration_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestTenantHook_NarrowsUserGetByID(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	orgRepo := repo.NewOrganizationRepo(testDB)
	userRepo := repo.NewUserRepo(testDB)

	// Need superuser admin handle to set the tenant GUC during setup so RLS
	// permits inserts. Use the admin handle for org creation (no RLS) and the
	// app handle for user creation under a SET LOCAL transaction.
	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create org A: %v", err)
	}
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB); err != nil {
		t.Fatalf("create org B: %v", err)
	}

	// Insert one user per org, using admin handle so RLS doesn't block setup.
	userA := &model.User{Name: "User A", Email: ptrString("a@a"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, userA); err != nil {
		t.Fatalf("create user A: %v", err)
	}
	userB := &model.User{Name: "User B", Email: ptrString("b@b"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, userB); err != nil {
		t.Fatalf("create user B: %v", err)
	}

	// As app_role with tenant=A in ctx, looking up userB.ID must come back NotFound.
	ctxA := tenant.With(ctx, orgA.ID)
	if _, err := userRepo.GetByID(ctxA, userB.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("cross-tenant GetByID leaked: want ErrNotFound, got %v", err)
	}
	// And userA should be visible under tenant=A.
	if _, err := userRepo.GetByID(ctxA, userA.ID); err != nil {
		t.Fatalf("same-tenant GetByID failed: %v", err)
	}
}

func TestTenantHook_NarrowsUserGetByEmail(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB)

	// Same email collision is allowed because the unique index is partial AND scoped
	// in a future plan; today we just use distinct emails.
	uA := &model.User{Name: "A", Email: ptrString("hook-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	uB := &model.User{Name: "B", Email: ptrString("hook-b@x"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uA)
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uB)

	userRepo := repo.NewUserRepo(testDB)
	ctxA := tenant.With(ctx, orgA.ID)
	if _, err := userRepo.GetByEmail(ctxA, "hook-b@x"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("cross-tenant GetByEmail leaked: want ErrNotFound, got %v", err)
	}
}

// Ensure the hook does not interfere with the organizations table (no organization_id col).
func TestTenantHook_OrganizationLookupUnaffected(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	org := &model.Organization{Name: "Z", Slug: "z", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	// With or without tenant, organization should be findable through admin and through app handle.
	got, err := repo.NewOrganizationRepo(testAdmin).GetBySlug(ctx, "z")
	if err != nil {
		t.Fatalf("admin GetBySlug: %v", err)
	}
	if got.Slug != "z" {
		t.Fatalf("got slug %q, want z", got.Slug)
	}
	// app handle uses GetBySlugAdmin (cross-tenant lookup) — must work without tenant ctx.
	gotApp, err := repo.NewOrganizationRepo(testDB).GetBySlugAdmin(ctx, "z")
	if err != nil {
		// Expected to fail because organizations isn't tenant-scoped but app_role
		// has SELECT permission. The hook should not add organization_id filter
		// because the organizations model has no such column. If it does, that's
		// the bug this test catches.
		_ = gotApp
		t.Fatalf("app GetBySlugAdmin failed: %v", err)
	}

	// And the hook should not have added an "organization_id" filter on the
	// organizations table. Hook is a no-op when ctx has no tenant; with tenant
	// in ctx Bun will fail with "column does not exist" if we get this wrong.
	uniqueOrg := uuid.New()
	ctxScoped := tenant.With(ctx, uniqueOrg)
	if _, err := repo.NewOrganizationRepo(testDB).GetBySlugAdmin(ctxScoped, "z"); err != nil {
		t.Fatalf("scoped lookup on organizations should not be hook-modified: %v", err)
	}
}
```

- [ ] **Step 2: Run, expect PASS for the first two tests; the third test pins the design choice**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test -run TestTenantHook ./internal/repo/... -v 2>&1 | tail -20
```

Expected: all three PASS.

If the third test (`TestTenantHook_OrganizationLookupUnaffected`) FAILS with `column "organization_id" of relation "organizations" does not exist`, the hook is over-aggressive — it must check whether the model has an `organization_id` field before adding the filter. In that case, refine the hook with a check; see Step 3.

- [ ] **Step 3 (only if Step 2 fails on the third test): Refine the hook to detect missing column**

If the third test fails, replace `api/internal/db/hooks.go`:

```go
// api/internal/db/hooks.go
package db

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

type TenantHook struct{}

var _ bun.QueryHook = TenantHook{}

func (TenantHook) BeforeQuery(ctx context.Context, e *bun.QueryEvent) context.Context {
	orgID, ok := tenant.From(ctx)
	if !ok {
		return ctx
	}
	if !modelHasOrgID(e.IQuery) {
		return ctx
	}
	switch q := e.IQuery.(type) {
	case *bun.SelectQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.UpdateQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	case *bun.DeleteQuery:
		q.Where("? = ?", bun.Ident("organization_id"), orgID)
	}
	return ctx
}

func (TenantHook) AfterQuery(ctx context.Context, e *bun.QueryEvent) {}

// modelHasOrgID reports whether the underlying Bun model defines an
// "organization_id" column. The hook only injects a filter for models that do.
func modelHasOrgID(q bun.Query) bool {
	type modelOwner interface{ GetModel() bun.Model }
	mo, ok := q.(modelOwner)
	if !ok {
		return false
	}
	tm, ok := mo.GetModel().(bun.TableModel)
	if !ok {
		return false
	}
	for _, f := range tm.Table().Fields {
		if f.SQLName == "organization_id" {
			return true
		}
	}
	return false
}
```

Re-run Step 2; both unit tests in `hooks_test.go` and the three integration tests should PASS.

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/repo/tenant_hook_integration_test.go api/internal/db/hooks.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "test(repo): TenantHook narrows by org; skips models without organization_id"
```

---

### Task 8: Repo test — RLS blocks cross-tenant reads even without the hook

**Files:**
- Create: `api/internal/repo/rls_integration_test.go`

This test connects as `mutqin_app` (RLS-subject), sets `app.current_tenant` to org A, and confirms a manual SQL `SELECT * FROM users` returns ONLY org A's users — not org B's.

- [ ] **Step 1: Write the test**

```go
// api/internal/repo/rls_integration_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestRLS_BlocksCrossTenantSelect(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "rls-a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create org A: %v", err)
	}
	orgB := &model.Organization{Name: "B", Slug: "rls-b", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB); err != nil {
		t.Fatalf("create org B: %v", err)
	}

	uA := &model.User{Name: "UA", Email: ptrString("rls-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	uB := &model.User{Name: "UB", Email: ptrString("rls-b@x"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uA); err != nil {
		t.Fatalf("create user A: %v", err)
	}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uB); err != nil {
		t.Fatalf("create user B: %v", err)
	}

	// Run a transaction on the app handle, set tenant=A, count users.
	tx, err := testDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant = '"+orgA.ID.String()+"'"); err != nil {
		t.Fatalf("set local: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("RLS leaked: count=%d, want 1 (only org A's user)", count)
	}

	// And switching tenant to a random uuid yields zero rows.
	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant = '"+uuid.New().String()+"'"); err != nil {
		t.Fatalf("set local 2: %v", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count again: %v", err)
	}
	if count != 0 {
		t.Fatalf("RLS leaked: count=%d, want 0 for a random tenant", count)
	}
}

func TestRLS_NoTenantSet_ReturnsNoRows(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "rls-c", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	uA := &model.User{Name: "UA", Email: ptrString("rls-c@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uA)

	// app handle, no SET LOCAL — current_setting('app.current_tenant', true) returns
	// empty string, the cast to uuid fails silently, USING clause yields NULL,
	// and Postgres treats NULL as not satisfying the predicate => 0 rows.
	var count int
	if err := testDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("RLS without tenant should yield 0 rows, got %d", count)
	}
}
```

- [ ] **Step 2: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test -run TestRLS ./internal/repo/... -v 2>&1 | tail -15
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/repo/rls_integration_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "test(rls): blocks cross-tenant select; empty tenant yields zero rows"
```

---

### Task 9: HTTP middleware — request_id

**Files:**
- Create: `api/internal/middleware/request_id.go`
- Create: `api/internal/middleware/request_id_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run, expect compile failure**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/middleware/...
```

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/request_id.go
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type requestIDKey struct{}

const requestIDHeader = "X-Request-ID"

// RequestID returns middleware that ensures every request carries a stable
// request ID. The ID comes from the incoming X-Request-ID header if present,
// otherwise a fresh UUID is generated. The ID is placed in ctx and echoed back
// in the response headers so clients and downstream services see the same one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(requestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext returns the request ID stored by RequestID middleware,
// or "" if not present.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey{}).(string)
	return v
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/middleware/... -v
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/middleware/request_id.go api/internal/middleware/request_id_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "middleware: request_id (generate or propagate X-Request-ID)"
```

---

### Task 10: HTTP middleware — structured logger

**Files:**
- Create: `api/internal/middleware/logger.go`
- Create: `api/internal/middleware/logger_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run, expect compile failure**

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/logger.go
package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// Logger returns middleware that emits a structured "http request" log line
// per request, including method, path, status, duration_ms, and request_id.
// Pass the application's slog.Logger; nil disables logging.
func Logger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if logger == nil {
				next.ServeHTTP(w, r)
				return
			}
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)
			logger.LogAttrs(r.Context(), slog.LevelInfo, "http request",
				slog.String("request_id", RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/middleware/...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/middleware/logger.go api/internal/middleware/logger_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "middleware: structured access logger with request_id"
```

---

### Task 11: HTTP middleware — Tenant resolver

**Files:**
- Create: `api/internal/middleware/tenant.go`
- Create: `api/internal/middleware/tenant_test.go`

The resolver inspects in order: `X-Tenant-Slug` header, then `Host` (extract `{slug}.<base>` if present), then leaves ctx untouched. For each candidate slug it asks an injected `OrgLookup` to map slug→UUID; if found, the org UUID goes into ctx via `tenant.With`.

- [ ] **Step 1: Write the failing tests**

```go
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
	req.Host = "mutqin.app" // apex, no subdomain
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
```

- [ ] **Step 2: Run, expect compile failure**

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/tenant.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// OrgLookup resolves a tenant slug to its organization UUID.
type OrgLookup interface {
	GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error)
}

// Tenant returns middleware that resolves the tenant from the request and
// injects the organization UUID into ctx via tenant.With.
//
// Resolution priority: X-Tenant-Slug header, then Host subdomain (only if Host
// matches "<slug>.<baseHost>"). If a slug is found but maps to nothing, the
// request is rejected with 404. If no slug is present at all, the chain
// proceeds with an unscoped ctx (e.g. apex host or admin path).
func Tenant(lookup OrgLookup, baseHost string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := r.Header.Get("X-Tenant-Slug")
			if slug == "" {
				slug = slugFromHost(r.Host, baseHost)
			}
			if slug == "" {
				next.ServeHTTP(w, r)
				return
			}
			id, err := lookup.GetOrgIDBySlug(r.Context(), slug)
			if err != nil {
				http.Error(w, "tenant not found", http.StatusNotFound)
				return
			}
			ctx := tenant.With(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// slugFromHost returns the leftmost subdomain when host matches "*.baseHost".
// Returns "" for apex, base-host-only, or any non-matching host (e.g. localhost).
func slugFromHost(host, baseHost string) string {
	host = strings.ToLower(strings.Split(host, ":")[0])
	baseHost = strings.ToLower(baseHost)
	if host == baseHost {
		return ""
	}
	suffix := "." + baseHost
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	return strings.TrimSuffix(host, suffix)
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/middleware/... -v 2>&1 | tail -20
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/middleware/tenant.go api/internal/middleware/tenant_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "middleware: tenant resolver (X-Tenant-Slug + Host subdomain)"
```

---

### Task 12: HTTP middleware — RLS context (`SET LOCAL`)

The RLS middleware wraps the request body in a transaction whose first statement is `SET LOCAL app.current_tenant`. If a tenant is in ctx, the transaction commits at the end if the handler returned without writing an error status; if no tenant is in ctx, the middleware passes through.

> **Note on architecture:** This middleware does not know about Bun's `*bun.DB` connection — instead, it sets the GUC at the SQL level via a transaction so that any subsequent query on that conn (via Bun, raw `database/sql`, or anything else) inherits the tenant. The Plan B implementation uses a per-request transaction; later plans may swap to a connection-level setting if performance requires.

**Files:**
- Create: `api/internal/middleware/rls.go`
- Create: `api/internal/middleware/rls_test.go`

- [ ] **Step 1: Write the failing test**

```go
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
```

- [ ] **Step 2: Run, expect compile failure**

- [ ] **Step 3: Implement**

```go
// api/internal/middleware/rls.go
package middleware

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/tenant"
)

// RLSTx is the minimal interface the middleware needs from a transaction:
// the ability to run "SET LOCAL app.current_tenant".
type RLSTx interface {
	ExecContext(ctx context.Context, query string, args ...any) error
}

// RLSRunner is the surface RLSContext middleware needs from a Bun handle.
// In production this is *bun.DB; tests provide a fake.
type RLSRunner interface {
	RunInTx(ctx context.Context, opts *bun.IDB, fn func(ctx context.Context, tx RLSTx) error) error
}

// RLSContext returns middleware that, when the request ctx carries a tenant,
// runs the rest of the request inside a Bun transaction whose first statement
// is `SET LOCAL app.current_tenant = '<uuid>'`. If no tenant is in ctx, the
// middleware is a no-op.
func RLSContext(runner RLSRunner) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := tenant.From(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			err := runner.RunInTx(r.Context(), nil, func(ctx context.Context, tx RLSTx) error {
				if err := tx.ExecContext(ctx, fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", id.String())); err != nil {
					return err
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return nil
			})
			if err != nil {
				http.Error(w, "tenant context error", http.StatusInternalServerError)
			}
		})
	}
}

// realRunner adapts *bun.DB to RLSRunner.
type realRunner struct{ db *bun.DB }

// NewBunRunner returns an RLSRunner backed by a real *bun.DB.
func NewBunRunner(db *bun.DB) RLSRunner { return &realRunner{db: db} }

func (r *realRunner) RunInTx(ctx context.Context, _ *bun.IDB, fn func(ctx context.Context, tx RLSTx) error) error {
	return r.db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, bunTx{tx: tx})
	})
}

type bunTx struct{ tx bun.Tx }

func (b bunTx) ExecContext(ctx context.Context, query string, args ...any) error {
	_, err := b.tx.ExecContext(ctx, query, args...)
	return err
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/middleware/...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/internal/middleware/rls.go api/internal/middleware/rls_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "middleware: RLSContext (per-request SET LOCAL app.current_tenant)"
```

---

### Task 13: Wire the middleware chain in `cmd/server/main.go`

**Files:**
- Modify: `api/cmd/server/main.go`
- Modify: `api/internal/config/config.go`

The server now needs:
- An `APP_DATABASE_URL` env var for the non-superuser connection
- The new middleware chain: `request_id → logger → tenant → rls`
- An `OrgLookup` adapter that wraps `OrganizationRepo.GetBySlugAdmin`

- [ ] **Step 1: Add `AppDatabaseURL` to config**

Open `api/internal/config/config.go`. Find the `Config` struct, add the new field after `DatabaseURL`:

```go
type Config struct {
	DatabaseURL    string
	AppDatabaseURL string
	JWTSecret      string
	Port           string

	BaseHost string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	R2AccountID string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
}
```

In `Load()`, after the `cfg := &Config{...}` block, populate the two new fields:

```go
	cfg.AppDatabaseURL = os.Getenv("APP_DATABASE_URL")
	if cfg.AppDatabaseURL == "" {
		cfg.AppDatabaseURL = cfg.DatabaseURL
	}
	cfg.BaseHost = os.Getenv("BASE_HOST")
	if cfg.BaseHost == "" {
		cfg.BaseHost = "mutqin.app"
	}
```

(`AppDatabaseURL` defaults to `DatabaseURL` so dev workflows that don't run migrations don't have to set both. `BaseHost` defaults to the production value but tests/Make can override.)

- [ ] **Step 2: Replace `api/cmd/server/main.go` with the wired-up version**

```go
// api/cmd/server/main.go
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

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/repo"
)

type orgLookupAdapter struct{ r *repo.OrganizationRepo }

func (a orgLookupAdapter) GetOrgIDBySlug(ctx context.Context, slug string) (uuid.UUID, error) {
	org, err := a.r.GetBySlugAdmin(ctx, slug)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	migrateUp := flag.Bool("migrate-up", false, "apply pending migrations and exit")
	migrateDown := flag.Bool("migrate-down", false, "roll back the most recent migration group and exit")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Admin/superuser handle — used for migrations and cross-tenant lookups
	// (e.g. resolving a slug to an org UUID before the tenant ctx is set).
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect admin database", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	switch {
	case *migrateUp:
		if err := migrate.Up(ctx, adminDB); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
		return
	case *migrateDown:
		if err := migrate.Down(ctx, adminDB); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	// App handle — non-superuser, RLS-subject. TenantHook auto-injects
	// WHERE organization_id = ? on tenant-scoped models.
	appDB, err := db.NewDB(ctx, cfg.AppDatabaseURL, false, db.TenantHook{})
	if err != nil {
		slog.Error("connect app database", "error", err)
		os.Exit(1)
	}
	defer appDB.Close()
	slog.Info("connected to database", "admin_dsn_redacted", redactDSN(cfg.DatabaseURL), "app_dsn_redacted", redactDSN(cfg.AppDatabaseURL))

	orgLookup := orgLookupAdapter{r: repo.NewOrganizationRepo(adminDB)}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS)
	r.Use(middleware.Tenant(orgLookup, cfg.BaseHost))
	r.Use(middleware.RLSContext(middleware.NewBunRunner(appDB)))

	r.Get("/api/v1/health", handler.Health())

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

// redactDSN returns the input DSN with the password component blanked.
// Used only for log lines so the DSN doesn't appear in the clear.
func redactDSN(s string) string {
	// crude: find the second ':' inside a userinfo "user:pass@" segment.
	at := -1
	for i := range s {
		if s[i] == '@' {
			at = i
			break
		}
	}
	if at < 0 {
		return s
	}
	colon := -1
	for i := 0; i < at; i++ {
		if s[i] == ':' && i > 0 && (i < 8 || s[:8] != "postgres") {
			colon = i
			break
		}
	}
	// fallback: if "postgres://" prefix, find the second ':' (after the scheme).
	if colon < 0 {
		count := 0
		for i := 0; i < at; i++ {
			if s[i] == ':' {
				count++
				if count == 2 {
					colon = i
					break
				}
			}
		}
	}
	if colon < 0 {
		return s
	}
	return s[:colon+1] + "***" + s[at:]
}
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go build ./...
```

- [ ] **Step 4: Verify existing health test still passes**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go test ./internal/handler/...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add api/cmd/server/main.go api/internal/config/config.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "cmd: wire RequestID/Logger/Tenant/RLSContext middleware; add APP_DATABASE_URL"
```

---

### Task 14: Update Makefile — `APP_DATABASE_URL` + `BASE_HOST`

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Replace Makefile**

```makefile
.PHONY: dev build db-up db-down migrate-up migrate-down test

DOCKER ?= docker
DATABASE_URL     ?= postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable
APP_DATABASE_URL ?= postgres://mutqin_app:mutqin_app@localhost:5432/mutqin?sslmode=disable
JWT_SECRET       ?= dev-secret
BASE_HOST        ?= mutqin.app

dev:
	cd api && DATABASE_URL=$(DATABASE_URL) APP_DATABASE_URL=$(APP_DATABASE_URL) JWT_SECRET=$(JWT_SECRET) BASE_HOST=$(BASE_HOST) go run ./cmd/server

build:
	cd api && go build -o bin/server ./cmd/server

db-up:
	$(DOCKER) compose up -d

db-down:
	$(DOCKER) compose down

migrate-up:
	cd api && DATABASE_URL=$(DATABASE_URL) JWT_SECRET=$(JWT_SECRET) go run ./cmd/server --migrate-up

migrate-down:
	cd api && DATABASE_URL=$(DATABASE_URL) JWT_SECRET=$(JWT_SECRET) go run ./cmd/server --migrate-down

test:
	cd api && go test ./...
```

- [ ] **Step 2: Smoke-test the new targets end to end**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b
make db-up
sleep 4
make migrate-up
make test
make db-down
```

Expected: all tests PASS.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b add Makefile
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b commit -m "make: add APP_DATABASE_URL and BASE_HOST env vars"
```

---

### Task 15: End-to-end smoke test

**Files:** none (verification only).

This run uses the dev DB (single Postgres process, two users) and verifies the full middleware chain end-to-end.

- [ ] **Step 1: Start dev DB**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b && make db-up && sleep 4
```

- [ ] **Step 2: Apply Bun migrations as superuser**

```bash
make migrate-up
```

Expected: log lines `connected to database` then `migrations applied group_id=1 count=3` (Plan A's two + Plan B's one).

- [ ] **Step 3: Verify schema + RLS**

```bash
docker exec mutqin-db psql -U mutqin -d mutqin -c "SELECT relname, relrowsecurity, relforcerowsecurity FROM pg_class WHERE relname IN ('users','invites','organizations','otp_codes') ORDER BY relname;"
```

Expected: `users` and `invites` have both flags `t`; `organizations` and `otp_codes` have both `f`.

- [ ] **Step 4: Verify role exists**

```bash
docker exec mutqin-db psql -U mutqin -d mutqin -c "SELECT rolname, rolsuper, rolcanlogin FROM pg_roles WHERE rolname = 'mutqin_app';"
```

Expected: one row with `rolname=mutqin_app rolsuper=f rolcanlogin=t`.

- [ ] **Step 5: Run the server**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && \
  DATABASE_URL='postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable' \
  APP_DATABASE_URL='postgres://mutqin_app:mutqin_app@localhost:5432/mutqin?sslmode=disable' \
  JWT_SECRET=dev-secret BASE_HOST=mutqin.app \
  go run ./cmd/server &
SERVER_PID=$!
sleep 3
curl -s -i http://localhost:8080/api/v1/health
echo
echo "---"
curl -s -i -H 'Host: ghost.mutqin.app' --resolve ghost.mutqin.app:8080:127.0.0.1 http://ghost.mutqin.app:8080/api/v1/health
kill -INT $SERVER_PID
wait $SERVER_PID 2>/dev/null
```

Expected:
- First curl (no tenant): 200 with body `{"data":{"status":"ok"}}`. Response includes `X-Request-ID` header.
- Second curl (`Host: ghost.mutqin.app`, slug doesn't exist): 404 (the tenant middleware rejects unknown slugs).

- [ ] **Step 6: Tear down**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b && make db-down
```

- [ ] **Step 7: Nothing to commit (verification only).**

---

### Task 16: Final verification + push

- [ ] **Step 1: Run the full test suite from a clean state**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go clean -testcache && go test ./...
```

Expected: handler PASS, tenant PASS, db PASS, middleware PASS, repo PASS (with all new tests).

- [ ] **Step 2: Run `go vet`**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b/api && go vet ./...
```

Expected: clean.

- [ ] **Step 3: Push the branch to both remotes**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b push -u origin feat/plan-b-tenant-isolation
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-b push gitlab feat/plan-b-tenant-isolation
```

- [ ] **Step 4: Open PRs/MRs**

GitHub PR base: `docs/backend-architecture-v2` (Plan A is already merged into this branch).

PR title: `Plan B — Tenant isolation defense-in-depth`

PR body:

```
## Summary
Three-layer tenant isolation enforcement on `users` and `invites`:
- Bun query hook auto-injects `WHERE organization_id = ?` from ctx.
- Postgres RLS policies fire at the storage layer (non-superuser `mutqin_app` role).
- HTTP middleware sets `app.current_tenant` per-request via SET LOCAL.

## Verification
- [x] `make db-up && make migrate-up` succeeds; Plan B migration creates `mutqin_app` role + RLS policies on `users`/`invites`.
- [x] `make test` passes (existing 7 + new tenant ctx + hook + RLS + middleware tests).
- [x] Smoke test: `Host: ghost.mutqin.app` → 404 from tenant resolver; apex host → 200 from health endpoint.

## Out of scope (deferred)
- JWT-based tenant resolution (Epic 1.2 OTP/JWT plan)
- Redis caching of slug→UUID lookup (in-memory adapter sufficient at 50-tenant target)
- RLS on tables that don't exist yet (halaqat, students, recitations, etc.)
```

GitLab MR base: same.

---

## Self-Review

**Spec coverage:**
- D8 layer 1 (Bun query hook): Tasks 2, 3, 7 ✓
- D8 layer 2 (Postgres RLS): Task 4 ✓
- D8 layer 3 (per-request SET LOCAL): Task 12 ✓
- Tenant resolution chain (Host + X-Tenant-Slug): Task 11 ✓
- Backend layering (request_id, logger, tenant, rls): Tasks 9–12 ✓
- Spec also mentions JWT `org` claim resolver path — explicitly out-of-scope (no JWT auth yet); will be added when auth lands.

**Placeholder scan:** No "TBD"/"TODO"/"implement later" / "add appropriate error handling" / "similar to Task N". Every step that produces code shows the code.

**Type / signature consistency:**
- `tenant.With(ctx, uuid.UUID) ctx`, `tenant.From(ctx) (uuid.UUID, bool)` — used consistently in Tasks 1, 2, 7, 11, 12, 13.
- `db.TenantHook{}` zero value, implements `bun.QueryHook` — used in Tasks 2, 3, 6, 13.
- `db.NewDB(ctx, dsn, debug, hooks ...bun.QueryHook)` — defined in Task 3, called in Tasks 6 (testDB+admin) and 13 (main.go) with matching variadic.
- `middleware.RequestID(http.Handler) http.Handler`, `RequestIDFromContext(ctx) string` — Task 9, used in Task 10.
- `middleware.Logger(*slog.Logger) func(http.Handler) http.Handler` — Task 10, used in Task 13.
- `middleware.OrgLookup` interface with `GetOrgIDBySlug(ctx, slug) (uuid.UUID, error)` — defined in Task 11, satisfied by `orgLookupAdapter` in Task 13.
- `middleware.Tenant(OrgLookup, baseHost) func(http.Handler) http.Handler` — Task 11, used in Task 13.
- `middleware.RLSContext(RLSRunner) func(http.Handler) http.Handler` — Task 12, used in Task 13.
- `middleware.NewBunRunner(*bun.DB) RLSRunner` — Task 12, used in Task 13.
- `repo.OrganizationRepo.GetBySlugAdmin(ctx, slug) (*model.Organization, error)` — Task 5, used in Task 13.

No mismatches found.

**Scope:** 16 tasks; each scoped to 2–10 minutes. Tasks 7 and 8 are the meaningful integration tests; everything else is small.
