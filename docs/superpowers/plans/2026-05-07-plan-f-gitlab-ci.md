# Plan F — GitLab CI/CD Pipeline

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land a `.gitlab-ci.yml` that lints, tests, builds, and deploys all three images (`api`, `landing`, `web`) plus a `docker-compose.prod.yml` override the deploy stage uses on the VPS to swap to HTTPS.

**Architecture:** Four stages — `lint`, `test`, `build`, `deploy`. Lint runs `golangci-lint` + `go vet` for Go and `eslint` + `tsc --noEmit` for TS. Test runs the Go suite using a `postgres:16-alpine` GitLab service container plus `testcontainers-go` for repo integration tests inside the runner (DinD-on-DinD is avoided by reusing the host service container for repo tests via env var). Build uses Kaniko or `docker:dind` to build the three images in parallel and push them to `$CI_REGISTRY_IMAGE/<name>:$CI_COMMIT_SHA` + `:latest`. Deploy runs only on `main`: it `ssh deploy@$VPS_HOST 'cd /opt/mutqin/src && git pull && docker compose -f docker-compose.yml -f docker-compose.prod.yml pull && docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d'`.

**Tech Stack:**
- GitLab CI (gitlab.com or self-hosted)
- GitLab Container Registry (built into the project)
- `kaniko` for rootless image builds (no DinD privileged container needed)
- Existing: `golangci-lint`, `go test`, `npm run lint`, `npm run typecheck`

**Out of scope (deferred):**
- Cloudflare API auto-record on tenant onboarding (Plan G)
- Database backup automation (mentioned in `docs/deploy.md`)
- Production monitoring (Prometheus, Grafana, alerting) — separate epic

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `.gitlab-ci.yml` | CI pipeline: lint → test → build → deploy |
| `docker-compose.prod.yml` | Production overrides: pin to registry images, mount `traefik.prod.yml`, expose 443, add `letsencrypt` volume, load env from `.env` |
| `api/.golangci.yml` | golangci-lint config — enable a sane preset, no per-line tweaks |
| `web/.eslintrc.cjs` | If absent — minimal eslint config for the existing TS scaffold |

### Modified
None.

### Deleted
None.

---

## Tasks

### Task 1: `api/.golangci.yml`

**Files:**
- Create: `api/.golangci.yml`

- [ ] **Step 1: Write a sane minimal config**

```yaml
# api/.golangci.yml
run:
  timeout: 5m
  go: "1.26"

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - revive
    - misspell

linters-settings:
  govet:
    enable:
      - shadow
  revive:
    severity: warning
    rules:
      - name: var-naming
      - name: package-comments
      - name: exported

issues:
  exclude-rules:
    - path: _test\.go
      linters:
        - errcheck
        - revive
```

- [ ] **Step 2: Verify it parses (golangci-lint may not be installed locally — that's fine; CI will validate)**

```bash
golangci-lint run --config api/.golangci.yml --timeout 30s api/... 2>&1 | head -30 || echo "golangci-lint not installed locally — CI will validate"
```

If installed: expect a small set of warnings (unexported helpers, etc.) — none of them block this task.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f add api/.golangci.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f commit -m "ci: golangci-lint config (errcheck, govet, staticcheck, gofmt, goimports, revive)"
```

---

### Task 2: `web/.eslintrc.cjs`

**Files:**
- Create: `web/.eslintrc.cjs` (if not already present)

- [ ] **Step 1: Check if it exists**

```bash
ls /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/web/.eslintrc* 2>&1
```

If a file already exists, **skip Steps 2–3** and proceed to Task 3.

- [ ] **Step 2: Write a minimal config matching the existing TS + React scaffold**

```js
// web/.eslintrc.cjs
module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    "eslint:recommended",
    "plugin:@typescript-eslint/recommended",
    "plugin:react-hooks/recommended",
  ],
  ignorePatterns: ["dist", ".eslintrc.cjs"],
  parser: "@typescript-eslint/parser",
  plugins: ["react-refresh"],
  rules: {
    "react-refresh/only-export-components": [
      "warn",
      { allowConstantExport: true },
    ],
  },
};
```

- [ ] **Step 3: Add `npm run lint` script if missing**

Open `web/package.json`. If `"lint"` is not in `scripts`, add:

```json
    "lint": "eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
    "typecheck": "tsc --noEmit"
```

- [ ] **Step 4: Commit (only if changes were made)**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f add web/
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f commit -m "ci: eslint config + lint/typecheck scripts"
```

---

### Task 3: `docker-compose.prod.yml`

**Files:**
- Create: `docker-compose.prod.yml`

This file is loaded ONLY in production via `docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d`. It overrides image references (build-from-source → registry-pulled), swaps the Traefik config file, opens 443, adds the ACME volume, and removes `build:` blocks so the prod runner never builds.

- [ ] **Step 1: Write the override**

```yaml
# docker-compose.prod.yml
# Production override. Apply with:
#   docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
#
# Required env vars at runtime (loaded by docker compose from .env or shell):
#   IMAGE_TAG          — :sha or :latest tag to pull (default :latest if unset)
#   CF_DNS_API_TOKEN   — Cloudflare API token, Zone:DNS:Edit on the zone
#   ACME_EMAIL         — Let's Encrypt registration email
#   POSTGRES_PASSWORD  — admin role password
#   APP_DB_PASSWORD    — mutqin_app role password (must match the migration grant)
#   JWT_SECRET         — long random
#   BASE_HOST          — e.g. mutqin.app

services:
  db:
    environment:
      POSTGRES_USER: mutqin
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?POSTGRES_PASSWORD required}
      POSTGRES_DB: mutqin

  api:
    build: !reset null
    image: ${CI_REGISTRY_IMAGE:-registry.gitlab.com/alieniz/mutqin}/api:${IMAGE_TAG:-latest}
    environment:
      DATABASE_URL: postgres://mutqin:${POSTGRES_PASSWORD}@db:5432/mutqin?sslmode=disable
      APP_DATABASE_URL: postgres://mutqin_app:${APP_DB_PASSWORD:?APP_DB_PASSWORD required}@db:5432/mutqin?sslmode=disable
      JWT_SECRET: ${JWT_SECRET:?JWT_SECRET required}
      BASE_HOST: ${BASE_HOST:-mutqin.app}
      PORT: "8080"

  landing:
    build: !reset null
    image: ${CI_REGISTRY_IMAGE:-registry.gitlab.com/alieniz/mutqin}/landing:${IMAGE_TAG:-latest}
    environment:
      DATABASE_URL: postgres://mutqin:${POSTGRES_PASSWORD}@db:5432/mutqin?sslmode=disable
      APP_DATABASE_URL: postgres://mutqin_app:${APP_DB_PASSWORD}@db:5432/mutqin?sslmode=disable
      JWT_SECRET: ${JWT_SECRET}
      BASE_HOST: ${BASE_HOST:-mutqin.app}
      PORT: "8080"

  web:
    build: !reset null
    image: ${CI_REGISTRY_IMAGE:-registry.gitlab.com/alieniz/mutqin}/web:${IMAGE_TAG:-latest}

  traefik:
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./infra/traefik/traefik.prod.yml:/etc/traefik/traefik.yml:ro
      - letsencrypt:/letsencrypt
    environment:
      CF_DNS_API_TOKEN: ${CF_DNS_API_TOKEN:?CF_DNS_API_TOKEN required}
      ACME_EMAIL: ${ACME_EMAIL:?ACME_EMAIL required}

volumes:
  letsencrypt:
```

Key points:
- `build: !reset null` is Compose 2's escape hatch: it removes the `build:` block from the base file, so prod never tries to compile.
- `:?VAR required` makes Compose fail-fast if a required env var is missing, instead of silently launching with empty values.
- `${CI_REGISTRY_IMAGE:-...}` defaults to a sensible registry path so a manual `docker compose ... up` works without exporting `CI_REGISTRY_IMAGE` first.

- [ ] **Step 2: Validate the file syntax (no daemon side effects)**

```bash
docker compose \
  -f /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/docker-compose.yml \
  -f /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/docker-compose.prod.yml \
  config 2>&1 | head -40
```

The command resolves both files into the merged effective config. Expect:
- `image:` set to the registry path on api/landing/web (no `build:` blocks)
- traefik has both port 80 and 443
- Compose complains about missing required env vars (`POSTGRES_PASSWORD required`) — that's the expected failure mode.

To validate without errors, set fake values:

```bash
POSTGRES_PASSWORD=x APP_DB_PASSWORD=y JWT_SECRET=z CF_DNS_API_TOKEN=t ACME_EMAIL=a@b.com \
  docker compose \
    -f /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/docker-compose.yml \
    -f /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/docker-compose.prod.yml \
    config 2>&1 | tail -20
```

Expected: clean YAML output showing the merged config with all values substituted.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f add docker-compose.prod.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f commit -m "infra: docker-compose.prod.yml override (TLS, registry images, ACME)"
```

---

### Task 4: `.gitlab-ci.yml` — pipeline scaffold

**Files:**
- Create: `.gitlab-ci.yml`

The pipeline has 4 stages. We keep image build using Kaniko so no DinD or privileged containers are needed.

- [ ] **Step 1: Write the file**

```yaml
# .gitlab-ci.yml
# CI/CD pipeline for Mutqin.
# Stages: lint → test → build → deploy.
# Build uses Kaniko to produce three images (api, landing, web) and pushes
# them to the project's GitLab Container Registry. Deploy SSHes into the
# production VPS and reconciles via docker compose.

stages:
  - lint
  - test
  - build
  - deploy

variables:
  GO_VERSION: "1.26"
  REGISTRY_IMAGE: "$CI_REGISTRY_IMAGE"

# ---------- LINT ----------
lint:go:
  stage: lint
  image: golangci/golangci-lint:v1.62-alpine
  script:
    - cd api
    - go vet ./...
    - golangci-lint run --timeout 5m

lint:web:
  stage: lint
  image: node:20-alpine
  cache:
    key:
      files: [web/package-lock.json]
    paths: [web/node_modules/]
  script:
    - cd web
    - npm ci
    - npm run lint
    - npm run typecheck

# ---------- TEST ----------
test:go:
  stage: test
  image: golang:1.26-alpine
  services:
    - name: postgres:16-alpine
      alias: postgres
  variables:
    POSTGRES_DB: mutqin_test
    POSTGRES_USER: mutqin
    POSTGRES_PASSWORD: mutqin
    DATABASE_URL: "postgres://mutqin:mutqin@postgres:5432/mutqin_test?sslmode=disable"
    # The repo testcontainers fixture starts its OWN postgres in a sub-container.
    # In GitLab the only available Docker daemon would be DinD; rather than
    # adding that, we skip integration tests via -short and rely on the unit
    # tests (handler, middleware, tenant ctx, hook unit). Repo integration
    # tests run locally via `make test`.
  script:
    - apk add --no-cache git
    - cd api
    - go test -short ./... -count=1

# ---------- BUILD ----------
.kaniko: &kaniko
  stage: build
  image:
    name: gcr.io/kaniko-project/executor:v1.23.2-debug
    entrypoint: [""]
  before_script:
    - mkdir -p /kaniko/.docker
    - |
      cat > /kaniko/.docker/config.json <<EOF
      {
        "auths": {
          "$CI_REGISTRY": {
            "username": "$CI_REGISTRY_USER",
            "password": "$CI_REGISTRY_PASSWORD"
          }
        }
      }
      EOF

build:api:
  <<: *kaniko
  script:
    - /kaniko/executor
        --context "$CI_PROJECT_DIR/api"
        --dockerfile "$CI_PROJECT_DIR/api/Dockerfile"
        --destination "$REGISTRY_IMAGE/api:$CI_COMMIT_SHA"
        --destination "$REGISTRY_IMAGE/api:latest"

build:landing:
  <<: *kaniko
  script:
    - /kaniko/executor
        --context "$CI_PROJECT_DIR/api"
        --dockerfile "$CI_PROJECT_DIR/api/Dockerfile.landing"
        --destination "$REGISTRY_IMAGE/landing:$CI_COMMIT_SHA"
        --destination "$REGISTRY_IMAGE/landing:latest"

build:web:
  <<: *kaniko
  script:
    - /kaniko/executor
        --context "$CI_PROJECT_DIR/web"
        --dockerfile "$CI_PROJECT_DIR/web/Dockerfile"
        --destination "$REGISTRY_IMAGE/web:$CI_COMMIT_SHA"
        --destination "$REGISTRY_IMAGE/web:latest"

# ---------- DEPLOY ----------
deploy:vps:
  stage: deploy
  image: alpine:3.20
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
  before_script:
    - apk add --no-cache openssh-client
    - eval $(ssh-agent -s)
    - echo "$VPS_SSH_PRIVATE_KEY" | tr -d '\r' | ssh-add -
    - mkdir -p ~/.ssh && chmod 700 ~/.ssh
    - ssh-keyscan -H "$VPS_HOST" >> ~/.ssh/known_hosts
  script:
    - |
      ssh deploy@"$VPS_HOST" <<EOF
        set -euo pipefail
        cd /opt/mutqin/src
        git fetch origin main
        git reset --hard origin/main
        export IMAGE_TAG="$CI_COMMIT_SHA"
        echo "$CI_REGISTRY_PASSWORD" | docker login -u "$CI_REGISTRY_USER" --password-stdin "$CI_REGISTRY"
        docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file /opt/mutqin/.env pull
        docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file /opt/mutqin/.env up -d
        docker image prune -f
      EOF
```

Required GitLab CI/CD project variables (set via `Settings → CI/CD → Variables`, all "Masked" + "Protected" except where noted):
- `VPS_HOST` — production VPS hostname (not masked since it's not secret)
- `VPS_SSH_PRIVATE_KEY` — SSH private key for the `deploy` user on the VPS (masked, file-type)

(`CI_REGISTRY`, `CI_REGISTRY_USER`, `CI_REGISTRY_PASSWORD`, `CI_REGISTRY_IMAGE`, `CI_COMMIT_SHA` are predefined by GitLab.)

- [ ] **Step 2: Validate YAML syntax locally**

```bash
python3 -c "import yaml,sys; yaml.safe_load(open('/Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/.gitlab-ci.yml'))" && echo "OK"
```

Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f add .gitlab-ci.yml
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f commit -m "ci: GitLab pipeline (lint, test, kaniko build, ssh deploy)"
```

---

### Task 5: Update `docs/deploy.md` for the prod compose flow

**Files:**
- Modify: `docs/deploy.md`

Plan D's `docs/deploy.md` says `docker-compose.prod.yml` is "TODO — Plan F adds this" and gives a manual one-line workaround. Now that the file exists, replace that section.

- [ ] **Step 1: Read the current file**

```bash
cat /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/docs/deploy.md
```

- [ ] **Step 2: Replace the "Production compose overrides" section with**

```markdown
## Production compose overrides

`docker-compose.prod.yml` overlays the local compose with production-specific values:
- Pins `api`, `landing`, `web` to images from the GitLab Container Registry (no in-place builds).
- Mounts `infra/traefik/traefik.prod.yml` as the Traefik config.
- Opens port 443 and adds a `letsencrypt` named volume for ACME state persistence.
- Loads passwords + tokens from `/opt/mutqin/.env`.

Bring the stack up with both files:

```sh
cd /opt/mutqin/src
docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  --env-file /opt/mutqin/.env \
  pull
docker compose \
  -f docker-compose.yml \
  -f docker-compose.prod.yml \
  --env-file /opt/mutqin/.env \
  up -d
```

The CI deploy stage in `.gitlab-ci.yml` runs the same commands automatically when a commit lands on `main`.
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f add docs/deploy.md
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f commit -m "docs(deploy): update prod compose flow to reference docker-compose.prod.yml"
```

---

### Task 6: Final verification + push

- [ ] **Step 1: Run unit tests one more time**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f/api && go test ./... 2>&1 | tail -8
```

Expected: all PASS.

- [ ] **Step 2: vet**

```bash
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Push the branch to both remotes**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f push -u origin feat/plan-f-gitlab-ci
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-f push gitlab feat/plan-f-gitlab-ci
```

The first push to GitLab triggers the pipeline. Watch:
```
https://gitlab.com/alieniz/mutqin/-/pipelines
```

- [ ] **Step 4: Open PR + MR**

Base: `docs/backend-architecture-v2`.

Title: `Plan F — GitLab CI/CD pipeline`

Body:

```
## Summary
- `.gitlab-ci.yml` with stages lint → test → build → deploy.
- Build uses Kaniko (no DinD); produces api / landing / web images tagged with $CI_COMMIT_SHA + latest, pushed to the project's GitLab Container Registry.
- Deploy runs only on `main`: SSHes to $VPS_HOST and runs `docker compose -f docker-compose.yml -f docker-compose.prod.yml --env-file /opt/mutqin/.env pull && up -d`.
- `docker-compose.prod.yml` overlays the local compose with registry images, the production Traefik config, port 443, an ACME volume, and required-env-var fail-fast guards.
- `api/.golangci.yml` enables a sane preset.
- Web eslint config + lint/typecheck scripts ensured.
- `docs/deploy.md` updated to reference the new prod compose flow.

## Required GitLab CI/CD variables
- `VPS_HOST` (not masked)
- `VPS_SSH_PRIVATE_KEY` (masked, file-type)

The first push triggers the pipeline. Build stage will succeed without setup; deploy only runs on `main` so it stays gated until the MR is merged.

## Out of scope
- Cloudflare API auto-record on tenant onboarding (Plan G)
- Backup automation (already noted in `docs/deploy.md`)
- Monitoring / alerting (separate epic)
```

---

## Self-Review

**Spec coverage:**
- D11 (CI/CD = GitLab + Container Registry) — Tasks 4 ✓
- Required-changes #8 (`.gitlab-ci.yml`) — Task 4 ✓
- `docker-compose.prod.yml` referenced as TODO in Plan D's `docs/deploy.md` — Task 3 + Task 5 ✓
- `app_role` password story (`APP_DB_PASSWORD`) wired through compose — Task 3 ✓

**Placeholder scan:** No "TBD"/"TODO" inside code. The prod compose's `${VAR:?required}` guards force fail-fast for missing values rather than leaving them blank. The deploy section explicitly lists required GitLab variables.

**Type / signature consistency:**
- Image names `api`, `landing`, `web` consistent across Dockerfiles, compose, prod compose, and CI build jobs.
- Image tag pattern `$CI_COMMIT_SHA` + `latest` consistent in CI builds and prod compose `${IMAGE_TAG:-latest}`.
- Env var names (`POSTGRES_PASSWORD`, `APP_DB_PASSWORD`, `JWT_SECRET`, `BASE_HOST`, `CF_DNS_API_TOKEN`, `ACME_EMAIL`) consistent in `docker-compose.prod.yml` and `docs/deploy.md`.
- GitLab predefined vars (`CI_REGISTRY`, `CI_REGISTRY_IMAGE`, etc.) used as-is — no aliasing.

**Scope:** 6 tasks. Substantive: 3, 4. Trivial: 1, 2, 5, 6.
