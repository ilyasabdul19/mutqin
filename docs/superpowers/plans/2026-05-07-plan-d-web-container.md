# Plan D — Web Container + Tenant-Aware Traefik Routing

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `web` container (`nginx:alpine` serving Vite `dist/`) and re-route Traefik so `/api/*` on any tenant subdomain reaches the api container while everything else lands on `web`. Local stack stays HTTP. Production TLS is configured behind a separate Traefik file gated on env vars (`CF_DNS_API_TOKEN` etc.) so it does not interfere with local dev.

**Architecture:** A multi-stage `web/Dockerfile` runs `npm ci && npm run build` (producing `dist/`) inside a `node:20-alpine` builder, then copies the static output into an `nginx:alpine` runtime with an SPA-fallback config. Traefik routing is restructured: a higher-priority router catches `PathPrefix(/api)` for both apex and `*.localhost`, falling through to `web` for any other path. A separate `infra/traefik/traefik.prod.yml` adds a websecure entrypoint and an ACME DNS-01 cert resolver using Cloudflare. Production deployment switches the Traefik file via env override; local dev uses the existing HTTP-only file.

**Tech Stack:**
- New: `node:20-alpine` builder, `nginx:alpine` runtime, Cloudflare ACME DNS-01 (configured but not exercised locally)
- Existing: Traefik v3.1, Docker Compose
- Frontend: Vite + React + PWA scaffold already exists at `web/` — Plan D only adds the Dockerfile + nginx config

**Out of scope (deferred):**
- Tenant theming / runtime CSS custom properties (Spec 2 — separate brainstorm)
- `cmd/landing` Go templates container at `pages.mutqin.app` (Plan E)
- GitLab CI building the web image (Plan F)
- Cloudflare API auto-create proxied A record on tenant onboarding (Plan G)
- Actual TLS issuance (requires real domain + CF token; documented + config provided, not verified)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `web/Dockerfile` | Multi-stage: node build → nginx runtime |
| `web/.dockerignore` | Exclude `node_modules`, `dist`, `.git`, etc. from build context |
| `web/nginx.conf` | nginx SPA-fallback config (`try_files $uri /index.html`) + cache headers on hashed assets |
| `infra/traefik/traefik.prod.yml` | Production Traefik static config — adds `websecure` entrypoint + ACME DNS-01 (Cloudflare) |
| `docs/deploy.md` | Production deployment notes — env vars (`CF_DNS_API_TOKEN`, `ACME_EMAIL`, `BASE_HOST`, etc.) and how to swap the Traefik file |

### Modified

| Path | Change |
|------|--------|
| `docker-compose.yml` | Add `web` service, restructure Traefik labels: api router gets `PathPrefix(/api)` + higher priority; web router gets default `Host()` matchers |
| `.gitignore` (root) | Ensure `web/dist/` and `web/node_modules/` are ignored (probably already, verify) |

### Deleted
None.

---

## Tasks

### Task 1: Add `web/Dockerfile`

**Files:**
- Create: `web/Dockerfile`

- [ ] **Step 1: Write the Dockerfile**

```dockerfile
# web/Dockerfile
# syntax=docker/dockerfile:1.7

# ---- build stage -----------------------------------------------------
FROM node:20-alpine AS build
WORKDIR /src

# Cache npm install — copy only manifest first.
COPY package.json package-lock.json ./
RUN npm ci

# Build the static bundle.
COPY . .
RUN npm run build

# ---- runtime stage ---------------------------------------------------
FROM nginx:alpine

# Replace default config with our SPA-aware one.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Copy the built assets.
COPY --from=build /src/dist /usr/share/nginx/html

EXPOSE 80
# Default nginx CMD already runs the daemon in the foreground.
```

- [ ] **Step 2: Build the image (this also exercises the npm ci + vite build path)**

```bash
docker build -t mutqin-web:dev /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d/web
```

Expected: clean build. Image size ~25–40 MB depending on bundle.

- [ ] **Step 3: Run the image standalone, hit the root**

```bash
docker run -d --rm --name mutqin-web-test -p 8090:80 mutqin-web:dev
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8090/
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8090/some/random/route
docker stop mutqin-web-test
```

Expected: both curls return `200` (the SPA fallback serves `index.html` for any unmatched path).

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add web/Dockerfile
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "build: web Dockerfile (node build → nginx runtime)"
```

---

### Task 2: Add `web/.dockerignore`

**Files:**
- Create: `web/.dockerignore`

- [ ] **Step 1: Write the file**

```
# web/.dockerignore
node_modules
dist
.git
.gitignore
*.log
*.md
.env
.env.*
```

- [ ] **Step 2: Rebuild and confirm context shrunk**

```bash
docker build -t mutqin-web:dev /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d/web 2>&1 | head -3
```

Expected: smaller "transferring context" line vs Task 1.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add web/.dockerignore
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "build: add web/.dockerignore"
```

---

### Task 3: Add `web/nginx.conf`

**Files:**
- Create: `web/nginx.conf`

- [ ] **Step 1: Write the config**

```nginx
# web/nginx.conf
# Single-page application fallback + cache rules for hashed Vite assets.

server {
    listen 80;
    server_name _;

    root /usr/share/nginx/html;
    index index.html;

    # gzip text-y stuff. Vite emits hashed filenames so caching is safe long.
    gzip on;
    gzip_comp_level 5;
    gzip_min_length 1024;
    gzip_types text/css application/javascript application/json image/svg+xml;

    # Long-cache hashed assets. Vite outputs files like /assets/index-abc123.js.
    location /assets/ {
        access_log off;
        expires 1y;
        add_header Cache-Control "public, immutable";
        try_files $uri =404;
    }

    # Service worker and manifest must NOT be cached by intermediaries beyond a short TTL,
    # otherwise PWA updates lag for hours.
    location = /sw.js {
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        try_files $uri =404;
    }
    location = /manifest.json {
        add_header Cache-Control "no-cache, no-store, must-revalidate";
        try_files $uri =404;
    }

    # SPA fallback — every other path returns index.html so client-side routing works.
    location / {
        try_files $uri /index.html;
    }
}
```

- [ ] **Step 2: Rebuild + run + verify SPA fallback returns the actual index.html (not a 404 page)**

```bash
docker build -t mutqin-web:dev /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d/web
docker run -d --rm --name mutqin-web-test -p 8090:80 mutqin-web:dev
sleep 2
curl -s http://localhost:8090/some/random/route | head -3
docker stop mutqin-web-test
```

Expected: HTML beginning with `<!doctype html>` and the `<title>Mutqin</title>` from `web/index.html`.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add web/nginx.conf
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "infra(web): nginx SPA-fallback config + asset caching"
```

---

### Task 4: Restructure Traefik labels in `docker-compose.yml`

**Files:**
- Modify: `docker-compose.yml`

The api router currently matches `PathPrefix(/api)` only — fine, but it's tied to no host. Once `web` is added that catches everything else, we need the api router to win on path priority. Use Traefik's `priority` label to keep api ahead of web on overlapping path matches.

- [ ] **Step 1: Replace the file contents**

```yaml
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
      # API router — matches /api on any host (apex localhost OR *.localhost). Higher priority
      # than the web router so the path prefix wins.
      - traefik.http.routers.api.rule=PathPrefix(`/api`)
      - traefik.http.routers.api.entrypoints=web
      - traefik.http.routers.api.priority=100
      - traefik.http.services.api.loadbalancer.server.port=8080
    networks:
      - mutqin

  web:
    build:
      context: ./web
      dockerfile: Dockerfile
    image: mutqin-web:dev
    container_name: mutqin-web
    labels:
      - traefik.enable=true
      - traefik.docker.network=mutqin
      # Web router — matches everything that isn't /api. Lower priority than api so api wins
      # on overlap.
      - traefik.http.routers.web.rule=PathPrefix(`/`)
      - traefik.http.routers.web.entrypoints=web
      - traefik.http.routers.web.priority=10
      - traefik.http.services.web.loadbalancer.server.port=80
    networks:
      - mutqin

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
    networks:
      - mutqin

volumes:
  pgdata:

networks:
  mutqin:
    name: mutqin
```

Key changes vs Plan C:
- Added `web` service with its own Dockerfile + Traefik labels.
- `api` router gains `priority=100`, `web` router gets `priority=10` so the api prefix always wins.
- `traefik` `depends_on` now includes web.

- [ ] **Step 2: Bring up the full stack**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d
docker compose up -d --build
sleep 12
docker compose ps
```

Expected: 5 containers (db, redis, api, web, traefik), db `healthy`, others `running`.

- [ ] **Step 3: Verify routing — root path goes to web**

```bash
curl -s http://localhost/ | head -3
```

Expected: `<!doctype html>` from the web container's `index.html`.

- [ ] **Step 4: Verify routing — /api/v1/health goes to api**

```bash
curl -s -i http://localhost/api/v1/health | head -8
```

Expected: 200 with `{"data":{"status":"ok"}}` and `X-Request-Id` header.

- [ ] **Step 5: Verify routing — tenant subdomain root goes to web**

```bash
curl -s -i -H "Host: alfalah.localhost" http://localhost/ | head -3
```

Expected: HTML doctype line. Web container served regardless of subdomain because it's host-agnostic and the api router's path prefix doesn't match.

- [ ] **Step 6: Verify routing — tenant subdomain /api with unknown slug returns 404**

```bash
curl -s -i -H "Host: ghost.localhost" http://localhost/api/v1/health | head -3
```

Expected: 404 (tenant resolver in the api rejects the unknown slug).

- [ ] **Step 7: Verify SPA deep-link fallback works through Traefik**

```bash
curl -s http://localhost/some/deep/route | head -3
```

Expected: HTML doctype (web container's `index.html` via nginx fallback).

- [ ] **Step 8: Tear down**

```bash
docker compose down
```

- [ ] **Step 9: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add docker-compose.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "infra: add web service to compose; api priority routing"
```

---

### Task 5: Add production Traefik config with TLS

**Files:**
- Create: `infra/traefik/traefik.prod.yml`

Production Traefik adds:
- A `websecure` entrypoint on `:443` with HTTP→HTTPS redirect from `:80`
- ACME DNS-01 cert resolver using Cloudflare API token (env var `CF_DNS_API_TOKEN`)
- Single wildcard cert for `mutqin.app` + `*.mutqin.app`

This file is NOT loaded locally. Production deploy script swaps the volume mount to use this file instead of `traefik.yml`.

- [ ] **Step 1: Write the config**

```yaml
# infra/traefik/traefik.prod.yml
# Production Traefik static configuration. Used in deploy by swapping the
# volume mount of /etc/traefik/traefik.yml to point here.
#
# Required env vars at runtime (set on the traefik container):
#   CF_DNS_API_TOKEN  — Cloudflare API token with Zone:DNS:Edit on the domain
#   ACME_EMAIL        — contact email Let's Encrypt records on the issued cert

api:
  dashboard: true
  # Dashboard not exposed on a public entrypoint in prod — reach it via SSH tunnel
  # to localhost:8080 inside the host. To expose via Traefik later, add a basic-auth
  # middleware and a router with hostname binding.
  insecure: false

entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
          permanent: true
  websecure:
    address: ":443"
    http:
      tls:
        certResolver: le
        domains:
          - main: "mutqin.app"
            sans:
              - "*.mutqin.app"

certificatesResolvers:
  le:
    acme:
      email: "${ACME_EMAIL}"
      storage: /letsencrypt/acme.json
      dnsChallenge:
        provider: cloudflare
        delayBeforeCheck: 10
        resolvers:
          - "1.1.1.1:53"
          - "8.8.8.8:53"

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

- [ ] **Step 2: Validate the YAML parses (no actual run — just syntax)**

```bash
docker run --rm -v /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d/infra/traefik:/etc/traefik:ro \
  traefik:v3.1 traefik --configFile=/etc/traefik/traefik.prod.yml --check 2>&1 | head -10
```

Expected: Traefik confirms the config is valid (or notes missing env vars — that's fine, we just want syntax-clean).

If `--check` is unavailable in v3.1, skip this step; Step 1's content is canonical.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add infra/traefik/traefik.prod.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "infra: production Traefik config (TLS via Cloudflare DNS-01)"
```

---

### Task 6: Add deployment notes

**Files:**
- Create: `docs/deploy.md`

A short ops doc telling future-you (or the operator) how to swap to production Traefik and what env vars to set.

- [ ] **Step 1: Write the doc**

```markdown
# Deployment Notes

This document covers the manual steps to deploy Mutqin to a production VPS using Docker Compose and Traefik with Let's Encrypt wildcard TLS via Cloudflare DNS-01.

## Prerequisites

- A VPS (e.g., Hetzner CX22) running Ubuntu 24.04 with Docker + Docker Compose v2 installed.
- A registered domain (`mutqin.app`) with DNS managed in Cloudflare.
- A Cloudflare API token with `Zone:DNS:Edit` permission on the zone.
- The wildcard A record `*.mutqin.app` set up in Cloudflare (gray-cloud / DNS-only is fine on the Free plan, or per-tenant proxied records auto-created later by Plan G's onboarding utility).

## One-time host setup

```sh
# On the VPS:
mkdir -p /opt/mutqin/letsencrypt
cd /opt/mutqin
git clone https://github.com/<owner>/mutqin.git src
cd src
```

## Required env vars

Create `/opt/mutqin/.env`:

```
DATABASE_URL=postgres://mutqin:<strongpw>@db:5432/mutqin?sslmode=disable
APP_DATABASE_URL=postgres://mutqin_app:<strongpw_app>@db:5432/mutqin?sslmode=disable
JWT_SECRET=<long-random>
BASE_HOST=mutqin.app
CF_DNS_API_TOKEN=<cloudflare token>
ACME_EMAIL=ops@mutqin.app
```

(`BASE_HOST` differs from local: tenant subdomains resolve via `*.mutqin.app` instead of `*.localhost`.)

## Production compose overrides

Use `docker-compose.prod.yml` (TODO — Plan F adds this) to override:
- The traefik volume mount → `infra/traefik/traefik.prod.yml`
- An additional volume `letsencrypt:/letsencrypt` for ACME persistence
- Open port `443` in addition to `80`

Until Plan F lands, you can manually adjust `docker-compose.yml` on the host or run a one-line override:

```sh
docker run -d --name mutqin-traefik \
  --network mutqin \
  -p 80:80 -p 443:443 \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v $(pwd)/infra/traefik/traefik.prod.yml:/etc/traefik/traefik.yml:ro \
  -v /opt/mutqin/letsencrypt:/letsencrypt \
  -e CF_DNS_API_TOKEN=$CF_DNS_API_TOKEN \
  -e ACME_EMAIL=$ACME_EMAIL \
  traefik:v3.1
```

## First TLS issuance

The first request to any host after starting Traefik triggers ACME DNS-01:

1. Traefik writes a `_acme-challenge.<domain>` TXT record via Cloudflare.
2. Lets Encrypt validates the record and issues a wildcard cert covering `mutqin.app` + `*.mutqin.app`.
3. The cert is stored in `/letsencrypt/acme.json` (chmod 600 on first write).

Watch the logs:

```sh
docker logs -f mutqin-traefik | grep -i acme
```

A successful run logs `[INFO] ... Server responded with a certificate.`

## Renewal

Lets Encrypt issues 90-day certs. Traefik auto-renews ~30 days before expiry. No cron job needed.

## Backups

`pgdata` and `letsencrypt` volumes are the persistent state. Schedule:

```sh
0 3 * * * docker run --rm -v mutqin_pgdata:/data postgres:16-alpine pg_dump -U mutqin mutqin | gzip > /opt/backups/mutqin-$(date +\%F).sql.gz
```

(Adjust path for your VPS. Cloudflare R2 sync is in Plan G alongside onboarding automation.)
```

- [ ] **Step 2: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d add docs/deploy.md
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d commit -m "docs: production deployment notes (TLS via DNS-01)"
```

---

### Task 7: Final verification + push

- [ ] **Step 1: Clean rebuild from a known state**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d
docker compose down -v
make up
sleep 18   # web build adds time
make ps
```

Expected: 5 containers, db `healthy`, api logs show migrations applied.

- [ ] **Step 2: Run the smoke matrix**

```bash
echo "--- root /  →  web ---"
curl -s -o /dev/null -w "code=%{http_code}\n" http://localhost/
echo "--- /api/v1/health  →  api ---"
curl -s -o /dev/null -w "code=%{http_code}\n" http://localhost/api/v1/health
echo "--- alfalah.localhost/  →  web ---"
curl -s -o /dev/null -w "code=%{http_code}\n" -H "Host: alfalah.localhost" http://localhost/
echo "--- ghost.localhost/api/v1/health  →  404 ---"
curl -s -o /dev/null -w "code=%{http_code}\n" -H "Host: ghost.localhost" http://localhost/api/v1/health
echo "--- /some/deep/route  →  web SPA fallback ---"
curl -s -o /dev/null -w "code=%{http_code}\n" http://localhost/some/deep/route
```

Expected output:
```
--- root /  →  web ---
code=200
--- /api/v1/health  →  api ---
code=200
--- alfalah.localhost/  →  web ---
code=200
--- ghost.localhost/api/v1/health  →  404 ---
code=404
--- /some/deep/route  →  web SPA fallback ---
code=200
```

- [ ] **Step 3: Tear down**

```bash
make down
```

- [ ] **Step 4: Run unit tests one more time**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d/api && go test ./... 2>&1 | tail -5
```

Expected: all PASS (Plan D doesn't touch api code).

- [ ] **Step 5: Push the branch to both remotes**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d push -u origin feat/plan-d-web-container
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-d push gitlab feat/plan-d-web-container
```

- [ ] **Step 6: Open PR + MR**

Base: `docs/backend-architecture-v2`.

Title: `Plan D — Web container + tenant-aware Traefik routing`

Body:

```
## Summary
- Multi-stage `web/Dockerfile` (node 20 build → `nginx:alpine` runtime, ~30 MB image).
- nginx SPA-fallback config + cache rules for hashed Vite assets, no-cache on `sw.js` / `manifest.json`.
- `docker-compose.yml` now has 5 services (added `web`); Traefik routes `/api/*` → api at priority 100 and everything else → web at priority 10 so the api prefix always wins.
- `infra/traefik/traefik.prod.yml` adds production websecure entrypoint + Lets Encrypt wildcard via Cloudflare DNS-01 (gated on `CF_DNS_API_TOKEN` + `ACME_EMAIL` — not exercised locally).
- `docs/deploy.md` documents env vars and the Traefik file swap for production.

## Verification (local)
- [x] `make up` starts 5 containers; `make ps` shows db `healthy`.
- [x] `curl http://localhost/` → web (HTML).
- [x] `curl http://localhost/api/v1/health` → api (`{"data":{"status":"ok"}}`).
- [x] `curl -H 'Host: alfalah.localhost' http://localhost/` → web.
- [x] `curl -H 'Host: ghost.localhost' http://localhost/api/v1/health` → 404 (tenant resolver).
- [x] `curl http://localhost/some/deep/route` → web SPA fallback returns 200.

## Out of scope (deferred)
- Tenant theming / runtime CSS custom properties (separate spec).
- `cmd/landing` Go-templates container at `pages.mutqin.app` (Plan E).
- GitLab CI building images (Plan F).
- Cloudflare API auto-create proxied A record on tenant onboarding (Plan G).
- Actual TLS issuance — config provided, requires real domain + CF token to verify.
```

---

## Self-Review

**Spec coverage:**
- D3 (`web` = `nginx:alpine` + Vite dist) — Tasks 1–3 ✓
- D6 (One LE wildcard via DNS-01) — Task 5 ✓ (config provided; verification deferred to deploy)
- Wildcard subdomain routing → web — Task 4 ✓ (host-agnostic priority routing achieves the same effect locally; `*.localhost` covered)
- Updated topology diagram from spec → matches Task 4's compose layout

**Placeholder scan:** No "TBD"/"TODO" placeholders in code. The deploy.md mentions `(TODO — Plan F adds this)` re: docker-compose.prod.yml, but that's an explicit forward reference, not a missing piece in this plan.

**Type / signature consistency:**
- Image tag `mutqin-web:dev` consistent across Dockerfile, compose service.
- Network name `mutqin` matches existing api/traefik labels.
- Traefik router names `api` (priority 100) and `web` (priority 10) consistent in compose labels.
- `infra/traefik/traefik.prod.yml` uses the same `mutqin` Docker network and same Docker provider as the local file.

**Scope:** 7 tasks. Trivial: 2, 3, 6 (config files). Substantive: 1, 4 (Dockerfile + compose restructure). Verification: 7.
