.PHONY: dev build db-up db-down migrate-up migrate-down migrate-create sqlc test

DOCKER ?= docker
DATABASE_URL ?= postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable

dev:
	cd api && go run ./cmd/server

build:
	cd api && go build -o bin/server ./cmd/server

db-up:
	$(DOCKER) compose up -d

db-down:
	$(DOCKER) compose down

migrate-up:
	migrate -path api/sql/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path api/sql/migrations -database "$(DATABASE_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir api/sql/migrations -seq $$name

sqlc:
	cd api/sql && sqlc generate

test:
	cd api && go test ./...
