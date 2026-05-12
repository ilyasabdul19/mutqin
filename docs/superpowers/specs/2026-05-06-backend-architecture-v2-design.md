---
title: Backend Architecture v2 — Containerized, Multi-Tenant, Bun ORM
status: draft
date: 2026-05-06
authors: [Suleiman]
supersedes_partial: docs/architecture.md
related: docs/prd.md, docs/epics.md
---

# Backend Architecture v2

Revised backend architecture for Mutqin. Replaces the Caddy-monolith deployment model and the sqlc + golang-migrate data layer with a containerized topology, Traefik gateway, and Bun ORM.

## Summary

| Dimension | Before (architecture.md) | After (this spec) |
|-----------|--------------------------|-------------------|
| Entry gateway | Caddy on host (single binary) | Traefik in container, label-based routing |
| Compute layout | One Go binary + static files on host | 6 containers: traefik, api, web, landing, postgres, redis |
| Frontend serving | Static files in `/var/www`, Caddy `file_server` | `web` container = `nginx:alpine` + Vite `dist/` |
| Public landing | Embedded in `api` binary as Go templates | Separate `landing` container, separate Go binary |
| Multi-tenant routing | Path-based, slug from JWT only | Wildcard subdomain `{slug}.mutqin.app` + JWT + Host-header fallback |
| Tenant theming | Out of scope | Runtime theme via `/api/v1/tenant/config` (deferred to Spec 2) |
| TLS | Caddy auto-HTTPS | Traefik + Let's Encrypt wildcard via DNS-01 (Cloudflare API token) |
| CDN | None | Cloudflare Free, proxied apex + per-tenant A records auto-created via API |
| Data access | sqlc (codegen, raw SQL) | Bun ORM (struct models + query builder + hooks) |
| Migrations | golang-migrate, sequential `.sql` files | Bun migrate, Go-file migrations |
| Tenant DB strategy | Shared DB, `organization_id` column, middleware enforcement | **Same**, plus Postgres RLS + Bun query hook (defense-in-depth) |
| CI/CD | Not specified | GitLab CI/CD, GitLab Container Registry |
| Deploy target | Single VPS | Single VPS now, K8s-ready label/CRD shape |

The shared-DB row-level tenant model from architecture.md is retained. All other infrastructure and data-layer choices are revised.

## Motivation

Three drivers, in priority order:

1. **Scale path** — Move to a topology that can be lifted into Kubernetes without rewriting the deployment story. Each service in its own container, Traefik labels mapping cleanly to Traefik IngressRoute CRDs.
2. **Multi-tenant identity at the gateway** — Centers should reach the platform at `{slug}.mutqin.app`. Wildcard subdomain routing gives clean per-tenant URLs and lets the gateway extract the slug into a header for downstream services. Sets up runtime theming in Spec 2.
3. **Developer ergonomics on the data layer** — The Mutqin team is small (one staff engineer + one frontend engineer). Bun's struct-based models and migrations let one developer move faster than sqlc's codegen + raw SQL workflow, and the shared codebase between `api` and `landing` benefits from one model package both binaries reuse.

## Decisions Locked

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Wildcard subdomain `{slug}.mutqin.app` + runtime theme (Approach A) | Lowest ops cost, fits 50-tenant target, runtime theming sufficient |
| D2 | API stays under `/api/*` on the tenant host (no `api.mutqin.app`) | Avoid CORS complexity, same-origin throughout |
| D3 | `web` container = `nginx:alpine` + Vite `dist/` | Tiny RAM, well-understood, easy to scale |
| D4 | `landing` is its own container (Go HTML templates) | Different audience, different cache profile, independent deploy |
| D5 | Cloudflare Free + per-tenant proxied A records auto-created via CF API on tenant onboarding | Free tier suffices; explicit records get proxying without paying for wildcard proxy |
| D6 | One Let's Encrypt wildcard cert via DNS-01 (Cloudflare API token) | Single cert covers apex + all tenant subdomains |
| D7 | Tenant DB strategy = shared DB, shared schema, `organization_id` column (Strategy A) | Cheapest, simplest cross-tenant queries for Super Admin dashboards |
| D8 | Tenant isolation defense-in-depth: Bun query hook + Postgres RLS + per-request `SET LOCAL` | Three independent layers, any one failure caught by the other two |
| D9 | ORM = Bun (replaces sqlc) | Struct-first models, built-in migrations, query hooks for tenant injection |
| D10 | Migrations = Bun migrate, Go-file migrations | Single tool, model and migration definitions live together |
| D11 | CI/CD = GitLab + GitLab Container Registry | User preference; simpler permissions story than GHCR for self-hosted runners later |
| D12 | Deploy target now = single Hetzner CX22 with Docker Compose; K8s shape preserved for later | Cost flat (~€5/mo), K8s migration is mechanical (labels → CRDs) |

## Architecture

### Container Topology

```
                Cloudflare (apex + per-tenant proxied A records)
                                │
                                ▼
                 ┌─────────────────────────────┐
                 │ traefik (entry, :80/:443)   │
                 │  TLS via LE wildcard DNS-01 │
                 │  trustedIPs = CF ranges     │
                 │  middlewares: request-id,   │
                 │   rate-limit, headers       │
                 └─┬─────────┬─────────┬───────┘
       Host(*.mutqin.app)    │         │ Host(pages.mutqin.app)
       + PathPrefix(/api/*)  │         │
                  │   Host(*.mutqin.app)
                  │   (else)
                  ▼          ▼          ▼
                ┌─────┐   ┌─────┐   ┌─────────┐
                │ api │   │ web │   │ landing │
                │ Go  │   │nginx│   │   Go    │
                └──┬──┘   └─────┘   └────┬────┘
                   │                     │
                   └────┬────────────────┘
                        ▼
                ┌──────────┐  ┌──────────┐
                │ postgres │  │  redis   │
                └──────────┘  └──────────┘
```

Six services. One VPS today. Same shape lifts to K8s as six Deployments + Services + IngressRoutes.

### Traefik Routing

Routing rules (priority resolved by Traefik Host/Path matching):

| Match | Service | Notes |
|-------|---------|-------|
| `Host(\`pages.mutqin.app\`)` | `landing` | Parent-facing HTML |
| `HostRegexp(\`{slug:[a-z0-9-]+}.mutqin.app\`) && PathPrefix(\`/api\`)` | `api` | Authenticated and public JSON API |
| `HostRegexp(\`{slug:[a-z0-9-]+}.mutqin.app\`)` | `web` | React PWA (everything else) |
| `Host(\`mutqin.app\`)` (apex) | `web` | Marketing fallback or 302 → pages |

Traefik middlewares applied to every router:

| Middleware | Purpose |
|------------|---------|
| `request-id` | Generate `X-Request-ID` if absent, propagate to backends |
| `rate-limit-public` | 60 req/min per IP on `/api/v1/public/*` and `landing` |
| `rate-limit-auth` | 600 req/min per JWT subject on authenticated `/api/v1/*` |
| `secure-headers` | HSTS, X-Frame-Options, X-Content-Type-Options |
| `tenant-slug-header` | Inject `X-Tenant-Slug` from URL slug capture group (api router only) |

### Multi-Tenant Model

**DB layout:** unchanged from architecture.md. Single Postgres database, single schema, every tenant table has `organization_id UUID NOT NULL REFERENCES organizations(id)`.

**Tenant resolution chain** (server-side, runs in middleware in this order):

| Priority | Source | Used when |
|----------|--------|-----------|
| 1 | JWT `org` claim | Request carries valid JWT |
| 2 | `X-Tenant-Slug` header (set by Traefik from URL) | Public/unauthenticated requests on a tenant subdomain |
| 3 | `Host` header parsing as fallback | Direct calls bypassing Traefik (dev only) |
| 4 | None | Non-tenant routes (`/api/v1/health`, `/api/v1/auth/*`) — middleware skips |

The middleware resolves slug → `organization_id` via Redis (60-second TTL) backed by Postgres lookup, and writes `organization_id` into `context.Context`.

**Defense-in-depth (3 layers):**

1. **Bun query hook** — `BeforeQuery` rewrites `SelectQuery`, `UpdateQuery`, `DeleteQuery` to add `WHERE organization_id = ?` whenever the request context carries a tenant. Manual `WHERE organization_id = ?` is forbidden in repo code (lint rule + code review).

2. **Postgres Row-Level Security** — Every tenant table gets:

   ```sql
   ALTER TABLE students ENABLE ROW LEVEL SECURITY;
   CREATE POLICY tenant_isolation ON students
     USING (organization_id = current_setting('app.current_tenant', true)::uuid);
   ```

   The application connects as a non-superuser role that cannot bypass RLS.

3. **Per-request connection setup** — On every request that has a resolved tenant, the middleware acquires a connection from the pool and runs:

   ```sql
   SET LOCAL app.current_tenant = '<uuid>';
   ```

   `LOCAL` scopes the setting to the current transaction; the connection returns to the pool clean.

If any one of these three fails (a missing hook, a missed policy, a missed `SET LOCAL`), the other two still block cross-tenant access.

### Backend Layering (Bun)

```
api/
├── cmd/
│   ├── server/main.go        # api binary entry
│   └── landing/main.go       # landing binary entry
├── internal/
│   ├── config/config.go      # env loading, single struct
│   ├── db/
│   │   ├── conn.go           # bun.DB factory + pgx pool
│   │   ├── hooks.go          # tenant query hook, slow-query log
│   │   └── tx.go             # RunInTx helper
│   ├── migrate/
│   │   ├── migrations/       # *.go migration files
│   │   └── runner.go         # invoked from cmd via --migrate-up flag
│   ├── model/                # Bun struct models (shared by api + landing)
│   ├── repo/                 # Bun queries, one file per aggregate
│   ├── service/              # business rules, ctx-aware
│   ├── handler/
│   │   ├── api/              # JSON handlers for api binary
│   │   └── landing/          # HTML handlers for landing binary
│   ├── middleware/
│   │   ├── request_id.go
│   │   ├── logger.go
│   │   ├── tenant.go         # slug → org_id resolution + ctx injection
│   │   ├── rls.go            # SET LOCAL app.current_tenant
│   │   ├── auth.go           # JWT validation
│   │   ├── role.go           # RBAC check
│   │   └── recoverer.go
│   ├── response/             # JSON envelope + error mapping
│   ├── templates/            # *.tmpl files for landing binary
│   └── server/router.go      # Chi router wiring
└── go.mod
```

**Layer rules:**

- Handlers parse requests, call services, write responses. They never touch the DB.
- Services hold business rules. They take `context.Context` and call repos. They never touch HTTP types.
- Repos hold Bun queries. They take `bun.IDB` (interface satisfied by both `bun.DB` and `bun.Tx`). They never hold business rules.
- Middleware is the only place that reads JWT and writes tenant context.

**Bun model example:**

```go
type Student struct {
    bun.BaseModel `bun:"table:students,alias:s"`

    ID             uuid.UUID `bun:",pk,type:uuid"`
    OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
    HalaqahID      uuid.UUID `bun:"halaqah_id,type:uuid"`
    Name           string    `bun:",notnull"`
    Age            int
    HifzLevel      string
    Status         string    `bun:",notnull,default:'active'"`
    CreatedAt      time.Time `bun:",nullzero,notnull,default:now()"`
}
```

**Tenant query hook:**

```go
type TenantHook struct{}

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
```

### Migrations

Bun migrate, Go files only. No raw SQL files in version control.

```
api/internal/migrate/migrations/
├── 20260506_000001_create_organizations.go
├── 20260506_000002_create_users_and_auth.go
├── 20260506_000003_create_halaqat_students.go
├── 20260506_000004_create_recitations.go
├── 20260506_000005_create_attendance.go
├── 20260506_000006_create_announcements_registrations.go
├── 20260506_000007_create_sync_conflicts.go
├── 20260506_000008_create_audit_log.go
├── 20260506_000009_enable_rls_on_tenant_tables.go
└── 20260506_000010_create_app_role_for_rls.go
```

Each migration file exports `Up(ctx, db)` and `Down(ctx, db)` functions using Bun's `db.NewCreateTable().Model(&Student{}).Exec(ctx)` style. The migration runner is invoked via `cmd/server` with `--migrate-up` (idempotent, fail-fast).

The two existing tables (`organizations`, `users`) currently created via raw SQL migrations are re-expressed as Bun migrations during cutover. The cutover plan is in the implementation plan, not this spec.

### Deployment + CI/CD

**Local dev:** `docker-compose.yml` defines all six services. `Makefile` exposes `make up`, `make down`, `make migrate`, `make seed`, `make logs`.

**CI/CD:** GitLab CI/CD with `.gitlab-ci.yml` at repo root. Stages: `lint → test → build → deploy`. Test stage runs Postgres as a service container. Build stage builds three images (`api`, `landing`, `web`) in parallel and pushes to GitLab Container Registry tagged `$CI_COMMIT_SHA` and `latest`. Deploy stage (only on `main`) SSHes to the VPS and runs `docker compose pull && docker compose up -d`.

**Secrets:** GitLab CI/CD variables (masked + protected): `VPS_HOST`, `SSH_PRIVATE_KEY`, `DATABASE_URL`, `JWT_SECRET`, `CF_API_TOKEN`, `RESEND_API_KEY`, `REDIS_URL`.

**Image tagging:** every build produces an immutable `$CI_COMMIT_SHA` tag and updates `latest`. Compose pulls `latest` on deploy. Rollback = pin SHA tag in compose env file and redeploy.

**Migrations on deploy:** `api` container runs `--migrate-up` on startup before serving traffic. Migration failure causes the new container to exit non-zero; the previous container keeps serving until the new one is healthy.

**K8s migration path (later, not in scope):** `docker-compose.yml` services map 1:1 to Kubernetes Deployments + Services + Traefik IngressRoute CRDs. Postgres moves to a managed provider. Redis moves to a managed provider or stays in-cluster. CF auto-records continue to work unchanged.

## Backend Dev Rules

Every new endpoint follows this fixed shape:

1. Add or update Bun model in `internal/model/`.
2. Generate a migration file in `internal/migrate/migrations/` with `Up` and `Down`.
3. Add or update repo methods in `internal/repo/` (Bun queries, no business logic).
4. Add or update service in `internal/service/` (business rules, calls repo).
5. Add handler in `internal/handler/api/` (parse request, call service, write response).
6. Wire route in `internal/server/router.go`.
7. Add tests:
   - `repo_test.go` — testcontainers Postgres
   - `service_test.go` — repo mocked
   - `handler_test.go` — `httptest` round-trip

Forbidden in repo and service code (CI lint rule):

- Manual `WHERE organization_id = ?` clauses (the hook handles it; manual filters mask hook bugs).
- Direct `bun.DB.Exec` outside repos.
- Cross-feature imports between feature packages.
- HTTP types (`http.Request`, `http.ResponseWriter`) outside `handler/`.

Logging: structured via `slog`. Every log line carries `request_id`, `org_id` (when present), `user_id` (when present).

Errors: services return typed errors (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`, `ErrValidation`). Handlers map them to HTTP status via `response.Error(w, err)`. Wire format unchanged from architecture.md (`{error: {code, message, details}}`).

Transactions: services start transactions via `db.RunInTx(ctx, func(ctx, tx) error { ... })`. Repos accept `bun.IDB`, so the same repo method works inside or outside a transaction.

## Required Changes from Current Architecture

The Epic 1.1 scaffolding (config, db pool, sqlc-generated `organizations` and `users`, CORS middleware, health handler) needs the following changes:

1. Replace `Caddy` reference in `Caddyfile` with Traefik configuration. Delete `Caddyfile`. Add `docker-compose.yml` with Traefik labels.
2. Drop sqlc dependency. Delete `api/internal/db/*.sql.go` generated files and `api/sql/queries/`.
3. Convert raw SQL migrations in `api/sql/migrations/` to Bun Go-file migrations under `api/internal/migrate/migrations/`. Existing schemas for `organizations` and `users` are re-expressed.
4. Add Bun dependency, `bun.DB` factory, query hook, RLS middleware.
5. Add `cmd/landing/main.go` and split landing handlers into `internal/handler/landing/`.
6. Add `web/Dockerfile` (multi-stage: node build → `nginx:alpine` serve dist) and `web/nginx.conf` for SPA fallback.
7. Add `api/Dockerfile` and `api/Dockerfile.landing` (multi-stage Go builds).
8. Add `.gitlab-ci.yml`.
9. Add Cloudflare API onboarding utility (script or Go subcommand) to create proxied A records on tenant creation. The utility lives in `api/cmd/onboard/` and is invoked from `service/organization.go` when a Super Admin creates a center.
10. Update `docs/architecture.md` to mark sections as "superseded by docs/backend-architecture.md" and add a pointer at the top. Author the new `docs/backend-architecture.md` as the active source of truth (this happens during implementation, not in this spec).

## Cost Impact

Steady-state monthly cost goes from ~€4 (CX22 + Caddy) to ~€5 (CX22 + Hetzner backups + R2 within free tier). Cloudflare Free covers DNS, CDN, and ACME. Email (Resend) is free up to 3000/month. The architecture revision is essentially cost-neutral.

Scale ladder (informational, not part of this spec's commitment):

| Trigger | Action | Monthly delta |
|---------|--------|---------------|
| Postgres working set > 1.5 GB or 50 → 200 tenants | CX22 → CX32 (4 vCPU, 8 GB) | +€4 |
| OTP volume > 3000/month | Resend free → Pro | +$20 |
| Per-tenant CDN caching needed | CF Free → CF Pro | +$20 |
| HA / multi-VPS | 2× CX22 + Hetzner Load Balancer | +€9 |
| Postgres off-box | Hetzner Managed PG (small) or Neon free tier | €0–€25 |

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Bun query hook silently misses an unusual query (e.g., raw SQL via `db.QueryContext`) | Postgres RLS + `SET LOCAL` catch the leak; lint rule forbids `db.QueryContext` outside `repo/` package |
| RLS policy misapplied when a new table is added | Migration template + checklist requires `ENABLE ROW LEVEL SECURITY` and policy on every tenant table; CI integration test asserts every tenant table has a policy |
| Cloudflare Free wildcard DNS records can't be proxied | Per-tenant explicit A record auto-created via CF API on tenant creation; falls back to gray-cloud if API call fails (alert ops) |
| Traefik LE rate limits during onboarding spikes | One wildcard cert covers all tenants; renewal is once per ~60 days |
| Postgres RLS bypass via superuser connection in dev | App connects as `app_role` (not superuser); superuser used only by migrations; dev tooling documents this |
| Migration failure on deploy leaves new container in crash loop | Compose health check keeps old container until new one healthy; rollback = pin previous SHA |
| Single-VPS failure | Documented but accepted; out of scope. HA path (multi-VPS + LB) noted in cost ladder |

## Out of Scope (Spec 2 and beyond)

- Frontend tenant theming runtime + CSS custom property system
- `/api/v1/tenant/config` endpoint contract and caching
- Cloudflare API onboarding utility implementation details (script structure, retry/backoff)
- Migration cutover plan from sqlc to Bun (lives in the implementation plan)
- Observability stack (Prometheus, Grafana, Loki) — current monitoring stays at Traefik dashboard + UptimeRobot
- M-Pesa, monitoring/alerting, CDN tuning beyond CF defaults
- Per-tenant physical isolation (Strategy C) — pattern documented for future use, not implemented

## Open Questions

None. All Q1–Q4 closed during brainstorm. Cutover sequencing belongs to the implementation plan.
