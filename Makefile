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
