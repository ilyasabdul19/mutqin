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
