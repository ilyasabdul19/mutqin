# Plan A — ORM Swap: sqlc → Bun

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace sqlc + golang-migrate with Bun ORM and Bun migrations for the existing `organizations`, `users`, `invites`, and `otp_codes` tables, while keeping the API serving `/api/v1/health` and adding repo tests proving CRUD parity.

**Architecture:** Introduce three new packages — `internal/model` (Bun struct models), `internal/migrate` (Go-file migrations using Bun's migrate runner), and `internal/repo` (data access with Bun query builders). Replace the `pgxpool` connection layer with `bun.DB` backed by `pgdriver`. Delete sqlc-generated code, sqlc query files, the raw-SQL migrations they shadowed, and the now-unused `pgx` and `golang-migrate` dependencies.

**Tech Stack:**
- `github.com/uptrace/bun` v1
- `github.com/uptrace/bun/dialect/pgdialect`
- `github.com/uptrace/bun/driver/pgdriver`
- `github.com/uptrace/bun/migrate`
- `github.com/uptrace/bun/extra/bundebug` (dev-only query logging)
- `github.com/google/uuid`
- `github.com/testcontainers/testcontainers-go/modules/postgres` (test only)

**Out of scope (deferred to later plans):**
- Tenant query hook, RLS policies, `SET LOCAL app.current_tenant` (Plan B)
- Containerization (Plan C)
- Wildcard subdomain routing, web container (Plan D)
- Landing container split (Plan E)
- GitLab CI/CD (Plan F)
- Cloudflare API onboarding (Plan G)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/model/organization.go` | Bun model for `organizations` table |
| `api/internal/model/user.go` | Bun model for `users` table |
| `api/internal/model/invite.go` | Bun model for `invites` table |
| `api/internal/model/otp.go` | Bun model for `otp_codes` table |
| `api/internal/migrate/migrations.go` | Migration registry singleton |
| `api/internal/migrate/runner.go` | `Up(ctx, db)` and `Down(ctx, db)` functions |
| `api/internal/migrate/migrations/20260506000001_create_organizations.go` | Creates `organizations` table |
| `api/internal/migrate/migrations/20260506000002_create_users_and_auth.go` | Creates `users`, `invites`, `otp_codes`, indexes, check constraints |
| `api/internal/repo/organization.go` | Bun queries for organizations |
| `api/internal/repo/user.go` | Bun queries for users |
| `api/internal/repo/repo_test.go` | Shared `TestMain` that boots a Postgres testcontainer, runs migrations, exposes `testDB *bun.DB` |
| `api/internal/repo/organization_test.go` | Repo tests for organization CRUD |
| `api/internal/repo/user_test.go` | Repo tests for user CRUD |

### Modified

| Path | Change |
|------|--------|
| `api/go.mod` | Add Bun + testcontainers + uuid; remove pgx + golang-migrate after cutover |
| `api/internal/db/conn.go` (new file replacing `db.go` + `pool.go`) | `bun.DB` factory, connectivity check |
| `api/internal/db/tx.go` | Rewrite `RunInTx` to use `bun.IDB` |
| `api/cmd/server/main.go` | Use `db.NewDB`, call `migrate.Up`, drop pgx pool / golang-migrate references |
| `Makefile` | Drop `sqlc` target; rewrite `migrate-up`/`migrate-down` to invoke `go run ./cmd/server --migrate-up` (and `--migrate-down`) |

### Deleted

| Path | Reason |
|------|--------|
| `api/internal/db/db.go` | sqlc boilerplate (`DBTX`, `Queries`) — replaced by `bun.DB` |
| `api/internal/db/pool.go` | pgx pool factory — replaced by `bun.DB` |
| `api/internal/db/querier.go` | sqlc generated interface |
| `api/internal/db/models.go` | sqlc generated structs — replaced by `model/` package |
| `api/internal/db/organizations.sql.go` | sqlc generated queries — replaced by `repo/organization.go` |
| `api/internal/db/users.sql.go` | sqlc generated queries — replaced by `repo/user.go` |
| `api/sql/queries/organizations.sql` | sqlc input — no longer needed |
| `api/sql/queries/users.sql` | sqlc input — no longer needed |
| `api/sql/migrations/001_create_organizations.up.sql` | Re-expressed in Bun migration |
| `api/sql/migrations/001_create_organizations.down.sql` | Re-expressed in Bun migration |
| `api/sql/migrations/002_create_users_and_auth.up.sql` | Re-expressed in Bun migration |
| `api/sql/migrations/002_create_users_and_auth.down.sql` | Re-expressed in Bun migration |
| `api/sql/sqlc.yaml` | sqlc config — sqlc removed |

> Migration placeholders `003_*` through `008_*` stay until the corresponding Bun migrations land in later plans. They contain only `-- placeholder` and are harmless.

---

## Tasks

### Task 1: Add new dependencies

**Files:**
- Modify: `api/go.mod`
- Modify: `api/go.sum`

- [ ] **Step 1: Add Bun packages**

Run from the repo root:

```bash
cd api && go get \
  github.com/uptrace/bun@v1 \
  github.com/uptrace/bun/dialect/pgdialect@v1 \
  github.com/uptrace/bun/driver/pgdriver@v1 \
  github.com/uptrace/bun/migrate@v1 \
  github.com/uptrace/bun/extra/bundebug@v1 \
  github.com/google/uuid
```

Expected: no output, `go.mod` updated with the new modules.

- [ ] **Step 2: Add testcontainers-go**

```bash
cd api && go get \
  github.com/testcontainers/testcontainers-go \
  github.com/testcontainers/testcontainers-go/modules/postgres
```

- [ ] **Step 3: Tidy module file**

```bash
cd api && go mod tidy
```

Expected: pgx and golang-migrate stay (still referenced by existing `db/` and `main.go` until Task 16).

- [ ] **Step 4: Verify build**

```bash
cd api && go build ./...
```

Expected: clean exit, no compile errors.

- [ ] **Step 5: Commit**

```bash
git add api/go.mod api/go.sum
git commit -m "deps: add Bun ORM, testcontainers-go, google/uuid"
```

---

### Task 2: Create new connection package

**Files:**
- Create: `api/internal/db/conn.go`

This file replaces `db.go` and `pool.go`. We will delete the old files in Task 16 (after the migration runner and main.go cutover are wired up).

- [ ] **Step 1: Write `conn.go`**

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

// NewDB opens a Bun DB backed by pgdriver, verifies connectivity, and returns
// the handle. The caller is responsible for closing it.
//
// debug=true installs bundebug to log every query — only enable in development.
func NewDB(ctx context.Context, databaseURL string, debug bool) (*bun.DB, error) {
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(databaseURL)))
	sqldb.SetMaxOpenConns(20)
	sqldb.SetMaxIdleConns(5)
	sqldb.SetConnMaxLifetime(30 * time.Minute)

	bdb := bun.NewDB(sqldb, pgdialect.New())
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

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/db
```

Expected: clean exit. Old `db.go`, `pool.go`, `tx.go`, `models.go`, `querier.go`, `*.sql.go` still compile alongside.

- [ ] **Step 3: Commit**

```bash
git add api/internal/db/conn.go
git commit -m "db: add Bun-backed NewDB connection factory"
```

---

### Task 3: Rewrite transaction helper to use Bun

**Files:**
- Modify: `api/internal/db/tx.go`

- [ ] **Step 1: Replace `tx.go` contents**

```go
// api/internal/db/tx.go
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

// RunInTx executes fn inside a database transaction. The transaction commits on
// nil return; rolls back on error. Repo methods accept bun.IDB so they work
// against either *bun.DB or bun.Tx.
func RunInTx(ctx context.Context, db *bun.DB, fn func(ctx context.Context, tx bun.Tx) error) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if err := fn(ctx, tx); err != nil {
			return fmt.Errorf("tx fn: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 2: Verify build (expects existing `RunTx` callers to break — there are none, but check)**

```bash
cd api && grep -rn "db.RunTx" .
```

Expected: no matches. (The old `RunTx` had no callers.)

```bash
cd api && go build ./...
```

Expected: clean exit.

- [ ] **Step 3: Commit**

```bash
git add api/internal/db/tx.go
git commit -m "db: rewrite tx helper for Bun"
```

---

### Task 4: Create `Organization` model

**Files:**
- Create: `api/internal/model/organization.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/organization.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Organization struct {
	bun.BaseModel `bun:"table:organizations,alias:o"`

	ID          uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Name        string     `bun:"name,notnull"`
	Slug        string     `bun:"slug,notnull,unique"`
	City        *string    `bun:"city"`
	Country     string     `bun:"country,notnull,default:'SO'"`
	Tier        string     `bun:"tier,notnull,default:'free'"`
	Status      string     `bun:"status,notnull,default:'active'"`
	LogoURL     *string    `bun:"logo_url"`
	Description *string    `bun:"description"`
	Schedule    []byte     `bun:"schedule,type:jsonb"`
	CreatedAt   time.Time  `bun:"created_at,notnull,default:now()"`
	UpdatedAt   time.Time  `bun:"updated_at,notnull,default:now()"`
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/model
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/model/organization.go
git commit -m "model: add Organization Bun model"
```

---

### Task 5: Create `User` model

**Files:**
- Create: `api/internal/model/user.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/user.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Phone          *string    `bun:"phone"`
	Email          *string    `bun:"email"`
	Name           string     `bun:"name,notnull"`
	Role           string     `bun:"role,notnull"`
	OrganizationID *uuid.UUID `bun:"organization_id,type:uuid"`
	Status         string     `bun:"status,notnull,default:'active'"`
	Language       string     `bun:"language,notnull,default:'ar'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/model
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/model/user.go
git commit -m "model: add User Bun model"
```

---

### Task 6: Create `Invite` model

**Files:**
- Create: `api/internal/model/invite.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/invite.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Invite struct {
	bun.BaseModel `bun:"table:invites,alias:i"`

	ID             uuid.UUID  `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	Role           string     `bun:"role,notnull"`
	Token          string     `bun:"token,notnull,unique"`
	ExpiresAt      time.Time  `bun:"expires_at,notnull"`
	UsedAt         *time.Time `bun:"used_at"`
	CreatedBy      uuid.UUID  `bun:"created_by,notnull,type:uuid"`
	CreatedAt      time.Time  `bun:"created_at,notnull,default:now()"`
}
```

- [ ] **Step 2: Commit**

```bash
git add api/internal/model/invite.go
git commit -m "model: add Invite Bun model"
```

---

### Task 7: Create `OtpCode` model

**Files:**
- Create: `api/internal/model/otp.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/otp.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type OtpCode struct {
	bun.BaseModel `bun:"table:otp_codes,alias:oc"`

	ID        uuid.UUID `bun:"id,pk,type:uuid,default:gen_random_uuid()"`
	Email     string    `bun:"email,notnull"`
	Code      string    `bun:"code,notnull"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
	Used      bool      `bun:"used,notnull,default:false"`
	CreatedAt time.Time `bun:"created_at,notnull,default:now()"`
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/model
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/model/otp.go
git commit -m "model: add OtpCode Bun model"
```

---

### Task 8: Create migration registry

**Files:**
- Create: `api/internal/migrate/migrations.go`

- [ ] **Step 1: Write the registry**

```go
// api/internal/migrate/migrations.go
package migrate

import (
	"github.com/uptrace/bun/migrate"
)

// Migrations is the global registry. Migration files self-register via init().
var Migrations = migrate.NewMigrations()
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/migrate
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/migrate/migrations.go
git commit -m "migrate: add migration registry"
```

---

### Task 9: Create migration runner

**Files:**
- Create: `api/internal/migrate/runner.go`

- [ ] **Step 1: Write the runner**

```go
// api/internal/migrate/runner.go
package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// Up applies all pending migrations.
func Up(ctx context.Context, db *bun.DB) error {
	migrator := migrate.NewMigrator(db, Migrations)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := migrator.Lock(ctx); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			slog.Error("unlock migrator", "error", err)
		}
	}()

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	if group.IsZero() {
		slog.Info("no new migrations to apply")
		return nil
	}
	slog.Info("migrations applied", "group_id", group.ID, "count", len(group.Migrations))
	return nil
}

// Down rolls back the most recent migration group.
func Down(ctx context.Context, db *bun.DB) error {
	migrator := migrate.NewMigrator(db, Migrations)
	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := migrator.Lock(ctx); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			slog.Error("unlock migrator", "error", err)
		}
	}()

	group, err := migrator.Rollback(ctx)
	if err != nil {
		return fmt.Errorf("rollback migrations: %w", err)
	}
	if group.IsZero() {
		slog.Info("no migrations to roll back")
		return nil
	}
	slog.Info("migrations rolled back", "group_id", group.ID, "count", len(group.Migrations))
	return nil
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/migrate
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/migrate/runner.go
git commit -m "migrate: add Up/Down runner functions"
```

---

### Task 10: Add organizations migration

**Files:**
- Create: `api/internal/migrate/migrations/20260506000001_create_organizations.go`

- [ ] **Step 1: Write the migration**

```go
// api/internal/migrate/migrations/20260506000001_create_organizations.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `
			CREATE TABLE organizations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    name TEXT NOT NULL,
			    slug TEXT NOT NULL UNIQUE,
			    city TEXT,
			    country TEXT NOT NULL DEFAULT 'SO',
			    tier TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'asaasi', 'pro')),
			    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
			    logo_url TEXT,
			    description TEXT,
			    schedule JSONB,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)
		`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE organizations`)
		return err
	})
}
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/migrate/migrations/20260506000001_create_organizations.go
git commit -m "migrate: add organizations table migration"
```

---

### Task 11: Add users + invites + otp_codes migration

**Files:**
- Create: `api/internal/migrate/migrations/20260506000002_create_users_and_auth.go`

- [ ] **Step 1: Write the migration**

```go
// api/internal/migrate/migrations/20260506000002_create_users_and_auth.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE users (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    phone TEXT,
			    email TEXT,
			    name TEXT NOT NULL,
			    role TEXT NOT NULL CHECK (role IN ('super_admin', 'center_admin', 'teacher')),
			    organization_id UUID REFERENCES organizations(id),
			    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'pending')),
			    language TEXT NOT NULL DEFAULT 'ar',
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE TABLE invites (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    role TEXT NOT NULL CHECK (role IN ('center_admin', 'teacher')),
			    token TEXT NOT NULL UNIQUE,
			    expires_at TIMESTAMPTZ NOT NULL,
			    used_at TIMESTAMPTZ,
			    created_by UUID NOT NULL REFERENCES users(id),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE TABLE otp_codes (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    email TEXT NOT NULL,
			    code TEXT NOT NULL,
			    expires_at TIMESTAMPTZ NOT NULL,
			    used BOOLEAN NOT NULL DEFAULT false,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_users_organization_id ON users(organization_id)`,
			`CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL`,
			`CREATE INDEX idx_invites_token ON invites(token)`,
			`CREATE INDEX idx_invites_organization_id ON invites(organization_id)`,
			`CREATE INDEX idx_otp_codes_email ON otp_codes(email)`,
			`ALTER TABLE users ADD CONSTRAINT users_org_role_check
			   CHECK (
			     (role = 'super_admin' AND organization_id IS NULL) OR
			     (role IN ('center_admin', 'teacher') AND organization_id IS NOT NULL)
			   )`,
			`ALTER TABLE users ADD CONSTRAINT users_contact_check
			   CHECK (phone IS NOT NULL OR email IS NOT NULL)`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`DROP TABLE otp_codes`,
			`DROP TABLE invites`,
			`DROP TABLE users`,
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
cd api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git add api/internal/migrate/migrations/20260506000002_create_users_and_auth.go
git commit -m "migrate: add users/invites/otp_codes migration"
```

---

### Task 12: Wire migration package import in cmd

**Files:**
- Modify: `api/cmd/server/main.go`

We need a side-effect import that drags the migrations package into the binary so `init()` registers each migration.

- [ ] **Step 1: Add the side-effect import**

Find the imports block in `main.go` and add:

```go
_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
```

The block becomes (only the imports section shown):

```go
import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler"
	"github.com/ilyas/mutqin-api/internal/middleware"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./...
```

Expected: clean exit. (Server still uses old golang-migrate runner — cutover happens in Task 17.)

- [ ] **Step 3: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "cmd: import bun migrations package for init() registration"
```

---

### Task 13: Add testcontainers test fixture

**Files:**
- Create: `api/internal/repo/repo_test.go`

This file's `TestMain` boots a Postgres testcontainer, applies Bun migrations against it, and exposes a package-level `testDB *bun.DB` for other test files in the same package.

- [ ] **Step 1: Write the fixture**

```go
// api/internal/repo/repo_test.go
package repo_test

import (
	"context"
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

var testDB *bun.DB

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

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	bdb, err := db.NewDB(ctx, dsn, false)
	if err != nil {
		log.Fatalf("connect bun: %v", err)
	}
	defer bdb.Close()

	if err := migrate.Up(ctx, bdb); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	testDB = bdb
	os.Exit(m.Run())
}
```

- [ ] **Step 2: Verify build (no test file targets exist yet, just compile)**

```bash
cd api && go test -run NoSuchTest ./internal/repo/...
```

Expected: `ok ... [no tests to run]` — package compiles, container not started.

- [ ] **Step 3: Commit**

```bash
git add api/internal/repo/repo_test.go
git commit -m "repo: add testcontainers Postgres fixture"
```

---

### Task 14: TDD organization repo — write the failing tests

**Files:**
- Create: `api/internal/repo/organization_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/repo/organization_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestOrganizationRepo_CreateAndGet(t *testing.T) {
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	org := &model.Organization{
		Name:    "Markaz Al-Falah",
		Slug:    "al-falah-create-get",
		Country: "SO",
		Tier:    "free",
		Status:  "active",
	}
	if err := r.Create(ctx, org); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if org.ID == uuid.Nil {
		t.Fatal("expected ID populated after Create")
	}

	got, err := r.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Slug != "al-falah-create-get" {
		t.Fatalf("want slug al-falah-create-get, got %s", got.Slug)
	}
}

func TestOrganizationRepo_GetBySlug(t *testing.T) {
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	org := &model.Organization{
		Name:    "Markaz Noor",
		Slug:    "noor-get-by-slug",
		Country: "SO",
		Tier:    "free",
		Status:  "active",
	}
	if err := r.Create(ctx, org); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.GetBySlug(ctx, "noor-get-by-slug")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.ID != org.ID {
		t.Fatalf("want ID %s, got %s", org.ID, got.ID)
	}
}

func TestOrganizationRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	_, err := r.GetByID(ctx, uuid.New())
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestOrganizationRepo_List(t *testing.T) {
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	for i, slug := range []string{"list-a", "list-b", "list-c"} {
		org := &model.Organization{
			Name:    "List Org " + slug,
			Slug:    slug,
			Country: "SO",
			Tier:    "free",
			Status:  "active",
		}
		_ = i
		if err := r.Create(ctx, org); err != nil {
			t.Fatalf("Create %s: %v", slug, err)
		}
	}

	got, err := r.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) < 3 {
		t.Fatalf("want at least 3 orgs, got %d", len(got))
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail (because the repo package does not exist yet)**

```bash
cd api && go test ./internal/repo/...
```

Expected: compile error — `package repo not found` or `undefined: repo.NewOrganizationRepo`.

- [ ] **Step 3: Commit the failing tests**

```bash
git add api/internal/repo/organization_test.go
git commit -m "test: failing organization repo tests"
```

---

### Task 15: Implement organization repo to make tests pass

**Files:**
- Create: `api/internal/repo/organization.go`
- Create: `api/internal/repo/errors.go`

- [ ] **Step 1: Write the shared errors file**

```go
// api/internal/repo/errors.go
package repo

import "errors"

// ErrNotFound is returned when a query expects a single row but finds none.
var ErrNotFound = errors.New("repo: not found")
```

- [ ] **Step 2: Write the organization repo**

```go
// api/internal/repo/organization.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type OrganizationRepo struct {
	db bun.IDB
}

func NewOrganizationRepo(db bun.IDB) *OrganizationRepo {
	return &OrganizationRepo{db: db}
}

func (r *OrganizationRepo) Create(ctx context.Context, org *model.Organization) error {
	_, err := r.db.NewInsert().Model(org).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert organization: %w", err)
	}
	return nil
}

func (r *OrganizationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	org := new(model.Organization)
	err := r.db.NewSelect().Model(org).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select organization by id: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	org := new(model.Organization)
	err := r.db.NewSelect().Model(org).Where("slug = ?", slug).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select organization by slug: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) List(ctx context.Context, limit, offset int) ([]model.Organization, error) {
	var orgs []model.Organization
	err := r.db.NewSelect().
		Model(&orgs).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return orgs, nil
}
```

- [ ] **Step 3: Run the tests to confirm they pass**

```bash
cd api && go test ./internal/repo/...
```

Expected: `PASS` for all four organization tests. (Postgres testcontainer boots once for the package; expect ~10s the first time.)

- [ ] **Step 4: Commit**

```bash
git add api/internal/repo/errors.go api/internal/repo/organization.go
git commit -m "repo: implement OrganizationRepo with Bun"
```

---

### Task 16: TDD user repo — write failing tests

**Files:**
- Create: `api/internal/repo/user_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// api/internal/repo/user_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func ptrString(s string) *string { return &s }
func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	ctx := context.Background()
	orgRepo := repo.NewOrganizationRepo(testDB)
	userRepo := repo.NewUserRepo(testDB)

	org := &model.Organization{Name: "User Org A", Slug: "user-org-a", Country: "SO", Tier: "free", Status: "active"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	u := &model.User{
		Name:           "Test Teacher",
		Email:          ptrString("teacher.a@example.com"),
		Role:           "teacher",
		OrganizationID: ptrUUID(org.ID),
		Status:         "active",
		Language:       "ar",
	}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == uuid.Nil {
		t.Fatal("expected ID populated after Create")
	}

	got, err := userRepo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email == nil || *got.Email != "teacher.a@example.com" {
		t.Fatalf("want email teacher.a@example.com, got %v", got.Email)
	}
}

func TestUserRepo_GetByEmail(t *testing.T) {
	ctx := context.Background()
	orgRepo := repo.NewOrganizationRepo(testDB)
	userRepo := repo.NewUserRepo(testDB)

	org := &model.Organization{Name: "User Org B", Slug: "user-org-b", Country: "SO", Tier: "free", Status: "active"}
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	u := &model.User{
		Name:           "Email Lookup",
		Email:          ptrString("lookup.b@example.com"),
		Role:           "center_admin",
		OrganizationID: ptrUUID(org.ID),
		Status:         "active",
		Language:       "ar",
	}
	if err := userRepo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := userRepo.GetByEmail(ctx, "lookup.b@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("want ID %s, got %s", u.ID, got.ID)
	}
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	userRepo := repo.NewUserRepo(testDB)

	_, err := userRepo.GetByID(ctx, uuid.New())
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
cd api && go test ./internal/repo/...
```

Expected: compile error — `undefined: repo.NewUserRepo`.

- [ ] **Step 3: Commit**

```bash
git add api/internal/repo/user_test.go
git commit -m "test: failing user repo tests"
```

---

### Task 17: Implement user repo

**Files:**
- Create: `api/internal/repo/user.go`

- [ ] **Step 1: Write the user repo**

```go
// api/internal/repo/user.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type UserRepo struct {
	db bun.IDB
}

func NewUserRepo(db bun.IDB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.NewInsert().Model(u).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u := new(model.User)
	err := r.db.NewSelect().Model(u).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select user by id: %w", err)
	}
	return u, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u := new(model.User)
	err := r.db.NewSelect().Model(u).Where("email = ?", email).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select user by email: %w", err)
	}
	return u, nil
}
```

- [ ] **Step 2: Run all repo tests**

```bash
cd api && go test ./internal/repo/...
```

Expected: PASS for all org and user tests.

- [ ] **Step 3: Commit**

```bash
git add api/internal/repo/user.go
git commit -m "repo: implement UserRepo with Bun"
```

---

### Task 18: Switch `cmd/server/main.go` to Bun

**Files:**
- Modify: `api/cmd/server/main.go`

This task does the cutover. After this, golang-migrate is unused, the pgx pool is unused, and the binary boots through Bun.

- [ ] **Step 1: Replace `main.go` contents**

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

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/handler"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
)

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

	bdb, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer bdb.Close()
	slog.Info("connected to database")

	switch {
	case *migrateUp:
		if err := migrate.Up(ctx, bdb); err != nil {
			slog.Error("migrate up", "error", err)
			os.Exit(1)
		}
		return
	case *migrateDown:
		if err := migrate.Down(ctx, bdb); err != nil {
			slog.Error("migrate down", "error", err)
			os.Exit(1)
		}
		return
	}

	if err := migrate.Up(ctx, bdb); err != nil {
		slog.Error("apply migrations on startup", "error", err)
		os.Exit(1)
	}

	r := chi.NewRouter()
	r.Use(middleware.CORS)
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
```

- [ ] **Step 2: Verify build**

```bash
cd api && go build ./...
```

Expected: clean exit. golang-migrate and pgx imports are gone from `main.go`.

- [ ] **Step 3: Verify the existing health test still passes**

```bash
cd api && go test ./internal/handler/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add api/cmd/server/main.go
git commit -m "cmd: cut over to Bun-backed db + Bun migrate runner"
```

---

### Task 19: Smoke-test the binary against the dev database

**Files:** none (verification only).

- [ ] **Step 1: Bring up the dev database**

```bash
make db-up
```

Expected: `mutqin-db` container running, port 5432 reachable.

- [ ] **Step 2: Run migrations explicitly**

```bash
DATABASE_URL=postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable \
JWT_SECRET=dev-secret \
go run ./api/cmd/server --migrate-up
```

Expected: log lines `connected to database` then `migrations applied group_id=1 count=2`, process exits 0.

- [ ] **Step 3: Verify schema in Postgres**

```bash
docker exec mutqin-db psql -U mutqin -d mutqin -c '\dt'
```

Expected: tables `bun_migrations`, `bun_migration_locks`, `organizations`, `users`, `invites`, `otp_codes`.

- [ ] **Step 4: Run the server, hit `/api/v1/health`**

In one terminal:
```bash
DATABASE_URL=postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable \
JWT_SECRET=dev-secret \
go run ./api/cmd/server
```

In another:
```bash
curl -s http://localhost:8080/api/v1/health
```

Expected response body:
```json
{"data":{"status":"ok"}}
```

- [ ] **Step 5: Stop the server (Ctrl-C) and tear down**

```bash
make db-down
```

- [ ] **Step 6: Nothing to commit (verification only)**

---

### Task 20: Delete sqlc generated files

**Files:**
- Delete: `api/internal/db/db.go`
- Delete: `api/internal/db/pool.go`
- Delete: `api/internal/db/querier.go`
- Delete: `api/internal/db/models.go`
- Delete: `api/internal/db/organizations.sql.go`
- Delete: `api/internal/db/users.sql.go`

- [ ] **Step 1: Confirm there are no callers**

```bash
cd api && grep -rn "db.New(" . | grep -v conn.go
cd api && grep -rn "db.Queries" .
cd api && grep -rn "db.NewPool" .
```

Expected: no matches outside `internal/db/conn.go` (which is the new factory and uses different names).

- [ ] **Step 2: Delete the files**

```bash
rm api/internal/db/db.go \
   api/internal/db/pool.go \
   api/internal/db/querier.go \
   api/internal/db/models.go \
   api/internal/db/organizations.sql.go \
   api/internal/db/users.sql.go
```

- [ ] **Step 3: Verify build + tests**

```bash
cd api && go build ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A api/internal/db
git commit -m "db: remove sqlc-generated files"
```

---

### Task 21: Delete sqlc query files, raw SQL migrations, and sqlc.yaml

**Files:**
- Delete: `api/sql/queries/organizations.sql`
- Delete: `api/sql/queries/users.sql`
- Delete: `api/sql/migrations/001_create_organizations.up.sql`
- Delete: `api/sql/migrations/001_create_organizations.down.sql`
- Delete: `api/sql/migrations/002_create_users_and_auth.up.sql`
- Delete: `api/sql/migrations/002_create_users_and_auth.down.sql`
- Delete: `api/sql/sqlc.yaml`

> Migration placeholder files `003_*` through `008_*` are intentionally left in place — they will be deleted as Plan B/C/etc. introduce real Bun migrations for those tables.

- [ ] **Step 1: Delete the files**

```bash
rm api/sql/queries/organizations.sql \
   api/sql/queries/users.sql \
   api/sql/migrations/001_create_organizations.up.sql \
   api/sql/migrations/001_create_organizations.down.sql \
   api/sql/migrations/002_create_users_and_auth.up.sql \
   api/sql/migrations/002_create_users_and_auth.down.sql \
   api/sql/sqlc.yaml
```

- [ ] **Step 2: Remove now-empty `queries` directory**

```bash
rmdir api/sql/queries
```

- [ ] **Step 3: Verify build + tests**

```bash
cd api && go build ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add -A api/sql
git commit -m "db: remove sqlc query files, raw 001/002 migrations, sqlc.yaml"
```

---

### Task 22: Drop unused dependencies

**Files:**
- Modify: `api/go.mod`
- Modify: `api/go.sum`

- [ ] **Step 1: Run `go mod tidy`**

```bash
cd api && go mod tidy
```

Expected: `go.mod` no longer lists `github.com/golang-migrate/migrate/v4`, `github.com/jackc/pgx/v5`, `github.com/lib/pq` (removed because nothing imports them now).

- [ ] **Step 2: Verify build + tests**

```bash
cd api && go build ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add api/go.mod api/go.sum
git commit -m "deps: drop pgx, golang-migrate, lib/pq (replaced by Bun)"
```

---

### Task 23: Update Makefile targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Replace the Makefile contents**

```makefile
# Makefile
.PHONY: dev build db-up db-down migrate-up migrate-down test

DOCKER ?= docker
DATABASE_URL ?= postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable
JWT_SECRET ?= dev-secret

dev:
	cd api && DATABASE_URL=$(DATABASE_URL) JWT_SECRET=$(JWT_SECRET) go run ./cmd/server

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

- [ ] **Step 2: Smoke-test the new targets**

```bash
make db-up
make migrate-up
make test
make db-down
```

Expected: all targets exit 0. Tests pass.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "make: drop sqlc, route migrate targets through cmd/server flags"
```

---

### Task 24: Final verification

- [ ] **Step 1: Run the full test suite from a clean state**

```bash
cd api && go clean -testcache && go test ./...
```

Expected: all packages PASS, including `internal/repo/...` (which spins up a Postgres testcontainer).

- [ ] **Step 2: Run `go vet`**

```bash
cd api && go vet ./...
```

Expected: clean.

- [ ] **Step 3: Verify final tree**

```bash
ls api/internal && ls api/sql
```

Expected:

```
api/internal:
  config db handler middleware migrate model repo response
api/sql:
  migrations
```

`api/sql/queries`, `api/sql/sqlc.yaml`, and the sqlc-generated `*.sql.go` files in `api/internal/db` are gone.

- [ ] **Step 4: Push the branch**

```bash
git push -u origin docs/backend-architecture-v2
```

(If this is happening on a different feature branch, push that branch instead.)

- [ ] **Step 5: Open the PR**

PR title: `Plan A — ORM swap: sqlc → Bun`

PR body checklist (verbatim):

```
## Summary
- Replaces sqlc with Bun for organizations, users, invites, otp_codes.
- Replaces golang-migrate with Bun's migrate runner; migrations are Go files.
- Adds repo package with testcontainers-based integration tests.
- Cuts unused dependencies (pgx, golang-migrate, lib/pq).

## Verification
- [ ] `make db-up && make migrate-up` succeeds
- [ ] `make test` passes
- [ ] `curl http://localhost:8080/api/v1/health` returns `{"data":{"status":"ok"}}`
- [ ] Schema includes bun_migrations, bun_migration_locks, organizations, users, invites, otp_codes

## Out of scope
- Tenant query hook + RLS (Plan B)
- Containerization, Traefik, web/landing split (Plans C, D, E)
```

---

## Self-Review

**Spec coverage:**
- D9 (ORM = Bun): Tasks 1, 4–7, 14–17 ✓
- D10 (Bun migrate): Tasks 8–12, 18 ✓
- Required-changes #2 (drop sqlc): Tasks 20, 21 ✓
- Required-changes #3 (re-express 001/002 in Bun): Tasks 10, 11, 21 ✓
- Required-changes #4 (Bun deps + factory): Tasks 1, 2 ✓
- Backend layering (`db/`, `model/`, `repo/`, `migrate/`): Tasks 2, 4–9, 15, 17 ✓

D7 (shared DB row-level), D8 (defense-in-depth), D11 (GitLab CI), D12 (containers), and the rest are explicitly out of scope and deferred to later plans.

**Placeholder scan:** No "TBD", "TODO", "implement later", "fill in details", or vague step descriptions. Every step that produces code shows the code; every step that runs a command shows the command and the expected result.

**Type / signature consistency:**
- `repo.NewOrganizationRepo(bun.IDB) *OrganizationRepo` — used in tests and main wiring; matches Task 15.
- `repo.NewUserRepo(bun.IDB) *UserRepo` — matches Task 17.
- `migrate.Up(ctx, *bun.DB) error` and `migrate.Down(ctx, *bun.DB) error` — declared in Task 9, called in Tasks 13 (test fixture) and 18 (main) with matching signatures.
- `db.NewDB(ctx, dsn, debug) (*bun.DB, error)` — declared in Task 2, called in Tasks 13 and 18 with matching signatures.
- `repo.ErrNotFound` — declared in Task 15, asserted in Tasks 14 and 16.
- Pointer-helper functions `ptrString`, `ptrUUID` — defined once in `user_test.go` (Task 16) and re-used only inside that file.

No mismatches found.

---

## Execution Handoff

**Plan complete and saved to `docs/superpowers/plans/2026-05-06-plan-a-orm-swap-sqlc-to-bun.md`. Two execution options:**

1. **Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — Execute tasks in this session using `executing-plans`, batch execution with checkpoints.

**Which approach?**
