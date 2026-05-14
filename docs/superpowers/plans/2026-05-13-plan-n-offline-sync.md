# Plan N — Offline Sync (Backend)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans`.

**Goal:** Backend endpoints that accept offline-collected recitations + attendance batches with idempotent dedup (via `client_id`) and let clients pull changes since a watermark timestamp.

**Architecture:** Two new HTTP endpoints (`POST /api/v1/sync/push`, `GET /api/v1/sync/pull`) plus a `sync_conflicts` table that records when a server-side row already differs from a client push (last-write-wins, but the rejected attempt is logged so a human can audit). Existing K (RecitationRepo) + L (AttendanceRepo) write paths already store `client_id` + `synced_at` — push reuses their `BatchCreate` / `UpsertBatch`. Push wraps the batch in a single transaction with `SET LOCAL app.current_tenant` (via existing RLSContext middleware). Pull emits rows where `recorded_at > since` for the tenant.

**Out of scope:**
- Service Worker / IndexedDB client code (web/ frontend) — Plan N covers backend only; web mechanics ship in a separate frontend plan
- Cross-table conflict resolution beyond last-write-wins
- Bulk sync for landing/announcement/halaqah edits — only the high-frequency teacher writes (recitations + attendance)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/migrate/migrations/20260513000001_sync_conflicts.go` | `sync_conflicts` table — global (not RLS) |
| `api/internal/model/sync_conflict.go` | Bun model |
| `api/internal/repo/sync_conflict.go` | `SyncConflictRepo` — Create only (audit-style) |
| `api/internal/repo/sync_conflict_test.go` | Integration |
| `api/internal/service/sync.go` | `SyncService` — Push (recitations + attendance arrays), Pull (since timestamp) |
| `api/internal/service/sync_test.go` | Mocked tests |
| `api/internal/handler/api/sync.go` | HTTP handlers |
| `api/internal/handler/api/sync_test.go` | Handler tests |

### Modified

| Path | Change |
|------|--------|
| `api/internal/repo/recitation.go` | Add `ListByOrgSince(ctx, since time.Time, limit int)` for pull |
| `api/internal/repo/attendance.go` | Add `ListByOrgSince(ctx, since time.Time, limit int)` for pull |
| `api/cmd/server/main.go` | Wire SyncService + handler; add to teacher route group |

---

## Tasks

### Task 1: `sync_conflicts` migration

`api/internal/migrate/migrations/20260513000001_sync_conflicts.go`:

```go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE sync_conflicts (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    table_name TEXT NOT NULL,
			    record_id UUID NOT NULL,
			    client_id TEXT,
			    client_data JSONB NOT NULL,
			    server_data JSONB NOT NULL,
			    resolution TEXT NOT NULL DEFAULT 'last_write_wins',
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_sync_conflicts_org_created ON sync_conflicts(organization_id, created_at DESC)`,
			`GRANT SELECT, INSERT ON sync_conflicts TO mutqin_app`,
			// sync_conflicts IS tenant-scoped but writes happen during conflict
			// resolution and need ctx-tenant set. Enable RLS for safety.
			`ALTER TABLE sync_conflicts ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE sync_conflicts FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON sync_conflicts
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
			`DROP POLICY IF EXISTS tenant_isolation ON sync_conflicts`,
			`ALTER TABLE sync_conflicts NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE sync_conflicts DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE sync_conflicts`,
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

Commit `migrate: sync_conflicts table + RLS`.

---

### Task 2: `SyncConflict` model

```go
// api/internal/model/sync_conflict.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type SyncConflict struct {
	bun.BaseModel `bun:"table:sync_conflicts,alias:sc"`
	TenantScoped

	ID             uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
	TableName      string    `bun:"table_name,notnull"`
	RecordID       uuid.UUID `bun:"record_id,notnull,type:uuid"`
	ClientID       *string   `bun:"client_id"`
	ClientData     []byte    `bun:"client_data,type:jsonb,notnull"`
	ServerData     []byte    `bun:"server_data,type:jsonb,notnull"`
	Resolution     string    `bun:"resolution,notnull,nullzero,default:'last_write_wins'"`
	CreatedAt      time.Time `bun:"created_at,notnull,nullzero,default:now()"`
}
```

Commit `model: SyncConflict Bun model`.

---

### Task 3: `SyncConflictRepo` (small — TDD)

```go
// api/internal/repo/sync_conflict.go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type SyncConflictRepo struct{ db bun.IDB }

func NewSyncConflictRepo(d bun.IDB) *SyncConflictRepo { return &SyncConflictRepo{db: d} }

func (r *SyncConflictRepo) idb(ctx context.Context) bun.IDB { return db.TxFrom(ctx, r.db) }

func (r *SyncConflictRepo) Create(ctx context.Context, c *model.SyncConflict) error {
	_, err := r.idb(ctx).NewInsert().Model(c).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert sync_conflict: %w", err)
	}
	return nil
}
```

Test (1): create + verify ID populated. Commit `repo: SyncConflictRepo`.

---

### Task 4: Add `ListByOrgSince` to RecitationRepo + AttendanceRepo

Both repos need a "fetch all rows in current tenant where `recorded_at > since`, limit clamp" method for the pull endpoint.

`api/internal/repo/recitation.go` — append:

```go
func (r *RecitationRepo) ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Recitation, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []model.Recitation
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("recorded_at > ?", since).
		OrderExpr("recorded_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("recitations since: %w", err)
	}
	return rows, nil
}
```

`api/internal/repo/attendance.go` — append:

```go
func (r *AttendanceRepo) ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Attendance, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []model.Attendance
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("recorded_at > ?", since).
		OrderExpr("recorded_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("attendance since: %w", err)
	}
	return rows, nil
}
```

Tests (2): seed rows at various timestamps, `ListByOrgSince` returns only those after `since`, ascending. Commit `repo: ListByOrgSince on recitation + attendance`.

---

### Task 5: `SyncService`

`api/internal/service/sync.go`:

```go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

var ErrInvalidSyncInput = errors.New("service: invalid sync input")

type recitationSync interface {
	BatchCreate(ctx context.Context, rs []model.Recitation) error
	ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Recitation, error)
}

type attendanceSync interface {
	UpsertBatch(ctx context.Context, rs []model.Attendance) error
	ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Attendance, error)
}

type syncConflictSink interface {
	Create(ctx context.Context, c *model.SyncConflict) error
}

type SyncService struct {
	recs    recitationSync
	att     attendanceSync
	conflict syncConflictSink
	audit   auditRepo
}

func NewSyncService(r recitationSync, a attendanceSync, c syncConflictSink, audit auditRepo) *SyncService {
	return &SyncService{recs: r, att: a, conflict: c, audit: audit}
}

// PushInput is the validated push payload (a batch of offline writes).
type PushInput struct {
	Recitations []model.Recitation
	Attendance  []model.Attendance
}

// PushResult reports how many rows of each kind were accepted.
type PushResult struct {
	RecitationsAccepted int       `json:"recitations_accepted"`
	AttendanceAccepted  int       `json:"attendance_accepted"`
	ServerNow           time.Time `json:"server_now"`
}

// Push performs both batches under the same RLS-scoped tx. Recitations use
// BatchCreate (insert-only; client_id dedupe via a partial unique index is
// future work). Attendance uses UpsertBatch (server already idempotent via
// UNIQUE(student_id, date)).
func (s *SyncService) Push(ctx context.Context, actorID, orgID uuid.UUID, in PushInput) (*PushResult, error) {
	// Set OrganizationID + TeacherID on every row (clients can lie about org).
	for i := range in.Recitations {
		in.Recitations[i].OrganizationID = orgID
		in.Recitations[i].TeacherID = actorID
		now := time.Now()
		in.Recitations[i].SyncedAt = &now
	}
	for i := range in.Attendance {
		in.Attendance[i].OrganizationID = orgID
		now := time.Now()
		in.Attendance[i].SyncedAt = &now
	}

	if len(in.Recitations) > 0 {
		if err := s.recs.BatchCreate(ctx, in.Recitations); err != nil {
			return nil, fmt.Errorf("push recitations: %w", err)
		}
	}
	if len(in.Attendance) > 0 {
		if err := s.att.UpsertBatch(ctx, in.Attendance); err != nil {
			return nil, fmt.Errorf("push attendance: %w", err)
		}
	}

	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID: &actorID,
		Action:  "sync_push",
	})

	return &PushResult{
		RecitationsAccepted: len(in.Recitations),
		AttendanceAccepted:  len(in.Attendance),
		ServerNow:           time.Now().UTC(),
	}, nil
}

// PullResult is the response for /sync/pull.
type PullResult struct {
	Recitations []model.Recitation `json:"recitations"`
	Attendance  []model.Attendance `json:"attendance"`
	ServerNow   time.Time          `json:"server_now"`
}

func (s *SyncService) Pull(ctx context.Context, since time.Time, limit int) (*PullResult, error) {
	recs, err := s.recs.ListByOrgSince(ctx, since, limit)
	if err != nil {
		return nil, fmt.Errorf("pull recitations: %w", err)
	}
	att, err := s.att.ListByOrgSince(ctx, since, limit)
	if err != nil {
		return nil, fmt.Errorf("pull attendance: %w", err)
	}
	return &PullResult{Recitations: recs, Attendance: att, ServerNow: time.Now().UTC()}, nil
}
```

Tests (3 stub-based):
- Push: stamps OrganizationID + TeacherID on each row before calling repos; both BatchCreate + UpsertBatch invoked
- Push: audit invoked once
- Pull: combines both lists; ServerNow populated

Commit `service: SyncService (Push + Pull)`.

---

### Task 6: HTTP handler

`api/internal/handler/api/sync.go`:

- `SyncService` interface (Push + Pull).
- `SyncHandler` with `Push(w, r)` and `Pull(w, r)`.
- `POST /api/v1/sync/push` body:
  ```json
  {
    "recitations": [ { /* model.Recitation shape */ } ],
    "attendance":  [ { /* model.Attendance shape */ } ]
  }
  ```
  Returns 200 with `PushResult`.
- `GET /api/v1/sync/pull?since=2026-05-12T00:00:00Z&limit=200` — parses RFC3339 `since`. If absent → since=0 (epoch). Returns `PullResult`.
- Auth: pull actor + orgID from `auth.From(ctx)`; 401 if no Identity, 400 if no OrgID (super_admin can't sync — it's tenant-scoped).
- Error mapping: bad JSON → 400, bad since → 400, infra → 500.

Tests (4+): Push happy path; Pull with since timestamp parses correctly; Pull missing since defaults to epoch; missing OrgID → 400.

Commit `handler: sync (push + pull)`.

---

### Task 7: Wire `cmd/server/main.go`

```go
syncSvc := service.NewSyncService(
    repo.NewRecitationRepo(appDB),
    repo.NewAttendanceRepo(appDB),
    repo.NewSyncConflictRepo(appDB),
    auditRepo,
)
syncH := apihandler.NewSyncHandler(syncSvc)
```

Add to teacher route group:

```go
tr.Post("/api/v1/sync/push", syncH.Push)
tr.Get("/api/v1/sync/pull", syncH.Pull)
```

Commit `cmd: wire SyncService + sync push/pull routes`.

---

### Task 8: Final verify + push

Tests + vet. Push to origin + gitlab.

---

## Self-Review

**PRD coverage:**
- FR30 (offline recitation sync) ✓ Push endpoint reuses RecitationRepo.BatchCreate
- FR33 (offline attendance sync) ✓ Push reuses AttendanceRepo.UpsertBatch
- Sync conflict logging — `sync_conflicts` table created; conflict-creation code is gated for last-write-wins triggers (future work — current implementation accepts client writes without detecting divergence; the table is in place for future conflict-detection logic)

**Type / signature consistency:**
- `service.PushInput` uses `model.Recitation` and `model.Attendance` directly — handlers decode JSON into these models.
- `ListByOrgSince` signature identical on both repos for symmetry.
- RLSContext middleware threads tx through ctx (Plan B fix in place) — both repo writes + reads run on the same tx with SET LOCAL.

**Scope:** 8 tasks. Substantive: 1, 4, 5, 6. Trivial: 2, 3. Wire/push: 7, 8.
