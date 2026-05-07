# Plan C — Containerize Core

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run the full backend stack (api + postgres + redis) behind Traefik via `docker compose up`. HTTP only locally on `localhost` and `*.localhost`; TLS deferred to Plan D.

**Architecture:** Add a multi-stage `api/Dockerfile` that builds a minimal, statically-linked Go binary into a distroless runtime image. Grow `docker-compose.yml` from one service (postgres) to four: postgres (with health check), redis, api (built from the new Dockerfile), traefik (entry router). Routing is label-based: Traefik watches the Docker socket, routes `localhost/api/*` and `{slug}.localhost/api/*` to the api container, and 404s anything else (web/landing land in later plans). Drop the unused `Caddyfile`.

**Tech Stack:**
- New: `traefik:v3` image, `redis:7-alpine`, `gcr.io/distroless/static-debian12` runtime base
- Existing: `postgres:16-alpine`
- Tooling: `docker compose v2`

**Out of scope (deferred):**
- TLS / wildcard cert / Cloudflare DNS-01 (Plan D)
- web (`nginx + Vite dist`) container (Plan D)
- landing (Go templates) container (Plan E)
- GitLab CI building images (Plan F)
- Cloudflare API auto-record onboarding (Plan G)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/Dockerfile` | Multi-stage Go build → distroless runtime |
| `api/.dockerignore` | Exclude build artifacts, `.git`, etc. from build context |
| `infra/traefik/traefik.yml` | Traefik static config (entrypoints, Docker provider, dashboard) |

### Modified

| Path | Change |
|------|--------|
| `docker-compose.yml` | Add health check on db; add `redis`, `api`, `traefik` services; expose Traefik dashboard at `:8081` |
| `Makefile` | Replace `db-up`/`db-down` with `up`/`down`/`logs`/`ps` covering the full stack; add `migrate-up`/`migrate-down` that exec into the running api container |

### Deleted

| Path | Reason |
|------|--------|
| `Caddyfile` | Replaced by Traefik; was an unused placeholder |

---

## Tasks

### Task 1: Add `api/Dockerfile`

**Files:**
- Create: `api/Dockerfile`

- [ ] **Step 1: Write the Dockerfile**

```dockerfile
# api/Dockerfile
# syntax=docker/dockerfile:1.7

# ---- build stage -----------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

# Build the static binary.
COPY . .
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime stage ---------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
COPY --from=build /out/server /app/server

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]
```

- [ ] **Step 2: Build the image**

```bash
docker build -t mutqin-api:dev /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c/api
```

Expected: clean build, ~20-30 MB final image.

- [ ] **Step 3: Verify the image runs (it will fail config because env not set — that's fine, just check it starts)**

```bash
docker run --rm mutqin-api:dev 2>&1 | head -3
```

Expected: log line about `failed to load config` (exit 1). The binary loaded; only the env is missing.

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add api/Dockerfile
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "build: multi-stage api Dockerfile (distroless static)"
```

---

### Task 2: Add `api/.dockerignore`

**Files:**
- Create: `api/.dockerignore`

- [ ] **Step 1: Write the file**

```
# api/.dockerignore
.git
.gitignore
*.md
bin/
vendor/
**/*_test.go
**/testdata/
.env
.env.*
```

- [ ] **Step 2: Rebuild and verify the build context shrank**

```bash
docker build -t mutqin-api:dev /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c/api 2>&1 | head -3
```

Expected: "Sending build context" line shows a smaller value than the prior build.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add api/.dockerignore
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "build: add api/.dockerignore"
```

---

### Task 3: Add Traefik static config

**Files:**
- Create: `infra/traefik/traefik.yml`

- [ ] **Step 1: Write the static config**

```yaml
# infra/traefik/traefik.yml
# Traefik static configuration. Local dev only — HTTP entrypoint, no TLS.
# Plan D will introduce a websecure entrypoint and ACME-issued wildcard cert.

api:
  dashboard: true
  insecure: true   # exposes the dashboard on the API entrypoint without auth — fine for local

entryPoints:
  web:
    address: ":80"

providers:
  docker:
    exposedByDefault: false
    network: mutqin

log:
  level: INFO
  format: json

accessLog:
  format: json
```

- [ ] **Step 2: Commit (no build to verify yet — used by docker-compose in Task 5)**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add infra/traefik/traefik.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "infra: add Traefik static config (HTTP-only local dev)"
```

---

### Task 4: Rewrite `docker-compose.yml` to include the full stack

**Files:**
- Modify: `docker-compose.yml`

- [ ] **Step 1: Replace contents**

```yaml
# docker-compose.yml
services:
  db:
    image: postgres:16-alpine
    container_name: mutqin-db
    environment:
      POSTGRES_USER: mutqin
      POSTGRES_PASSWORD: mutqin
      POSTGRES_DB: mutqin
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U mutqin -d mutqin"]
      interval: 2s
      timeout: 2s
      retries: 30
    networks:
      - mutqin

  redis:
    image: redis:7-alpine
    container_name: mutqin-redis
    command: ["redis-server", "--save", "", "--appendonly", "no"]
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 1s
      retries: 30
    networks:
      - mutqin

  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    image: mutqin-api:dev
    container_name: mutqin-api
    environment:
      DATABASE_URL: postgres://mutqin:mutqin@db:5432/mutqin?sslmode=disable
      APP_DATABASE_URL: postgres://mutqin_app:mutqin_app@db:5432/mutqin?sslmode=disable
      JWT_SECRET: dev-secret
      BASE_HOST: localhost
      PORT: "8080"
    depends_on:
      db:
        condition: service_healthy
    labels:
      - traefik.enable=true
      - traefik.docker.network=mutqin
      - traefik.http.routers.api.rule=PathPrefix(`/api`)
      - traefik.http.routers.api.entrypoints=web
      - traefik.http.services.api.loadbalancer.server.port=8080
    networks:
      - mutqin

  traefik:
    image: traefik:v3.1
    container_name: mutqin-traefik
    command:
      - --configFile=/etc/traefik/traefik.yml
    ports:
      - "80:80"      # web entrypoint
      - "8081:8080"  # dashboard (Traefik's API runs on 8080 inside the container)
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./infra/traefik/traefik.yml:/etc/traefik/traefik.yml:ro
    depends_on:
      - api
    networks:
      - mutqin

volumes:
  pgdata:

networks:
  mutqin:
    name: mutqin
```

Key points:
- Both `api` and `traefik` join the named `mutqin` network so Traefik can reach the api container by service name.
- `traefik.docker.network=mutqin` label tells Traefik which network to use when multiple are attached.
- Dashboard exposed at `localhost:8081` (`insecure: true` from Task 3 makes it work on the same entrypoint).
- The api container migrates the database on startup via `migrate.Up` (already wired in `cmd/server/main.go`), so no separate migrate service is needed.

- [ ] **Step 2: Bring up the stack**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c && docker compose up -d --build
```

Expected: 4 containers start (db healthy first, then redis + api + traefik). Build takes ~30s on a cold cache.

- [ ] **Step 3: Verify each container is up**

```bash
docker compose ps
```

Expected: db `healthy`, redis `healthy`, api `running`, traefik `running`.

- [ ] **Step 4: Verify the api logs show migrations applied + server starting**

```bash
docker compose logs api 2>&1 | tail -10
```

Expected: log lines `connected to database`, `migrations applied group_id=1 count=3` (or `no new migrations to apply` on a re-run), `server starting port=8080`.

- [ ] **Step 5: Hit the health endpoint via Traefik**

```bash
curl -s -i http://localhost/api/v1/health
```

Expected: `HTTP/1.1 200 OK`, response body `{"data":{"status":"ok"}}`, response includes `X-Request-Id` header.

- [ ] **Step 6: Hit a tenant subdomain — unknown slug → 404**

```bash
curl -s -i -H "Host: ghost.localhost" http://localhost/api/v1/health | head -5
```

Expected: `HTTP/1.1 404 Not Found` (tenant resolver rejects unknown slug).

- [ ] **Step 7: Tear down**

```bash
docker compose down
```

- [ ] **Step 8: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add docker-compose.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "infra: docker compose stack — db + redis + api + traefik"
```

---

### Task 5: Delete `Caddyfile`

**Files:**
- Delete: `Caddyfile`

- [ ] **Step 1: Verify nothing references it**

```bash
grep -rn "Caddyfile\|caddy" /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c --exclude-dir=docs --exclude-dir=.git
```

Expected: no matches outside `docs/` (architecture.md will still mention Caddy as historical context).

- [ ] **Step 2: Delete the file**

```bash
rm /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c/Caddyfile
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add -A Caddyfile
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "infra: remove Caddyfile (replaced by Traefik)"
```

---

### Task 6: Update `Makefile` for the full stack

**Files:**
- Modify: `Makefile`

The new targets manage the whole stack rather than just the database.

- [ ] **Step 1: Replace the file**

```makefile
.PHONY: dev build up down logs ps test migrate-up migrate-down

DOCKER ?= docker
DATABASE_URL     ?= postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable
APP_DATABASE_URL ?= postgres://mutqin_app:mutqin_app@localhost:5432/mutqin?sslmode=disable
JWT_SECRET       ?= dev-secret
BASE_HOST        ?= mutqin.app

# Local dev WITHOUT containers — runs the api against a host-mode db at localhost:5432.
dev:
	cd api && DATABASE_URL=$(DATABASE_URL) APP_DATABASE_URL=$(APP_DATABASE_URL) JWT_SECRET=$(JWT_SECRET) BASE_HOST=$(BASE_HOST) go run ./cmd/server

build:
	cd api && go build -o bin/server ./cmd/server

# Bring up the full stack (db, redis, api, traefik) via docker compose.
up:
	$(DOCKER) compose up -d --build

down:
	$(DOCKER) compose down

logs:
	$(DOCKER) compose logs -f

ps:
	$(DOCKER) compose ps

# Run migrations inside the api container. The api container also migrates on
# its own startup; this target is for manual re-runs without restarting it.
migrate-up:
	$(DOCKER) compose exec api /app/server --migrate-up

migrate-down:
	$(DOCKER) compose exec api /app/server --migrate-down

test:
	cd api && go test ./...
```

Key changes from the prior Makefile:
- `db-up` / `db-down` are gone. `up` / `down` cover the whole stack.
- `migrate-up` / `migrate-down` now `docker compose exec` into the running api container (the binary in the distroless image lives at `/app/server`).
- The standalone `dev` target stays for working against a host-mode local Postgres without rebuilding the image on every Go change.

- [ ] **Step 2: Smoke-test the new targets**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c
make up
sleep 8
make ps
curl -s http://localhost/api/v1/health
echo
make migrate-up
make down
```

Expected: `make up` brings 4 containers up healthy; health check returns `{"data":{"status":"ok"}}`; `make migrate-up` reports `no new migrations to apply` (idempotent); `make down` cleans up.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c add Makefile
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c commit -m "make: full-stack targets (up/down/logs/ps); migrate via docker compose exec"
```

---

### Task 7: Final verification + push

- [ ] **Step 1: Clean rebuild from a known state**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c
docker compose down -v   # also remove the pgdata volume
make up
sleep 12   # initial migration takes a few seconds
make ps
```

Expected: 4 containers, db healthy, api logs show `migrations applied group_id=1 count=3`.

- [ ] **Step 2: Confirm full smoke**

```bash
curl -s -i http://localhost/api/v1/health | head -8
echo
curl -s -i -H "Host: ghost.localhost" http://localhost/api/v1/health | head -3
```

Expected: 200 on the apex path, 404 for unknown slug.

- [ ] **Step 3: Confirm Traefik dashboard is reachable (sanity check)**

```bash
curl -s http://localhost:8081/api/version | head -2
```

Expected: JSON with Traefik version info.

- [ ] **Step 4: Tear down**

```bash
make down
```

- [ ] **Step 5: Run unit tests one more time (host-mode Go) to make sure nothing got broken**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c/api && go test ./... 2>&1 | tail -10
```

Expected: all PASS.

- [ ] **Step 6: Push the branch to both remotes**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c push -u origin feat/plan-c-containerize
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-c push gitlab feat/plan-c-containerize
```

- [ ] **Step 7: Open PR + MR**

PR/MR base: `docs/backend-architecture-v2`.

Title: `Plan C — Containerize core (Traefik + Docker Compose)`

Body:

```
## Summary
Containerize the full backend stack and route through Traefik:
- Multi-stage `api/Dockerfile` → distroless static runtime (~25 MB image).
- `docker-compose.yml` grows from 1 service (db) to 4: db (healthchecked) + redis + api + traefik.
- Traefik routes `localhost/api/*` and `*.localhost/api/*` to the api container via Docker provider labels.
- Dashboard at `localhost:8081`.
- HTTP only locally; TLS deferred to Plan D.

## Verification
- [x] `make up` starts the stack; `make ps` shows db `healthy`.
- [x] `curl http://localhost/api/v1/health` returns `{"data":{"status":"ok"}}` with `X-Request-Id`.
- [x] `curl -H 'Host: ghost.localhost' http://localhost/api/v1/health` returns 404.
- [x] `make migrate-up` is idempotent inside the running container.

## Out of scope (deferred)
- TLS / wildcard cert / Cloudflare DNS-01 (Plan D)
- web (`nginx + Vite`) container (Plan D)
- landing (Go templates) container (Plan E)
- GitLab CI image build (Plan F)
- Cloudflare API auto-record onboarding (Plan G)
```

---

## Self-Review

**Spec coverage:**
- D5 (Cloudflare auto-record) → out of scope here, Plan G
- D12 (single-VPS containerization with K8s shape preserved) → addressed: 4 services, named network, label-based routing — directly portable to k8s Deployments + IngressRoute CRDs
- "Drop Caddy" from Required-Changes #1 → Task 5
- Traefik service from architecture diagram → Task 4
- Redis service for slug→org cache (forward-looking) → Task 4

**Placeholder scan:** No "TBD"/"TODO"/"add error handling" placeholders. Every task gives exact files and commands.

**Type / signature consistency:**
- Postgres user/password in compose (`mutqin:mutqin`) matches the existing migration that creates `mutqin_app` (Plan B), so the dual-role connection works inside the container too.
- Image tag `mutqin-api:dev` referenced consistently in Dockerfile, compose, Makefile.
- Traefik network name `mutqin` matches the network declared in compose and the `traefik.docker.network=mutqin` label on api.
- Migration binary path `/app/server` matches the `WORKDIR /app` + `COPY --from=build /out/server /app/server` in the Dockerfile.

**Scope:** 7 tasks. Smaller than Plans A and B because the work is mostly config files; logic stays put.
