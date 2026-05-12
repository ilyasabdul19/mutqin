# Plan L — Attendance Tracking

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teachers mark daily attendance for their halaqah in bulk (all-present default, mark exceptions). Center admins query attendance by halaqah / student / date-range.

**Architecture:** `attendance` table — RLS-subject, unique on `(student_id, date)` so bulk-mark is idempotent via `INSERT ... ON CONFLICT`. `AttendanceRepo.UpsertBatch` is the primary write path; reads slice by halaqah-and-date or student-and-range. Service validates inputs and audits. Handlers expose a single bulk-mark endpoint plus three read endpoints.

**Tech Stack:** existing — Bun, Chi, RLS, established service/handler patterns.

**Out of scope (deferred):**
- Offline-collected attendance sync (FR33) — Plan N hooks into `UpsertBatch` via `client_id` dedupe
- Attendance trends / charting (Plan M)
- Notifications to parents on absence — separate epic

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/migrate/migrations/20260512000002_attendance.go` | `attendance` table + UNIQUE (student_id, date) + RLS + grants |
| `api/internal/model/attendance.go` | Bun model (embeds `TenantScoped`) |
| `api/internal/repo/attendance.go` | `AttendanceRepo` — UpsertBatch, ListByHalaqahDate, ListByStudent (date range), ListByOrg (date range + optional halaqah filter) — uses `r.idb(ctx)` |
| `api/internal/repo/attendance_test.go` | Integration |
| `api/internal/service/attendance.go` | Validation + audit + orchestration |
| `api/internal/service/attendance_test.go` | Mocked tests |
| `api/internal/handler/api/attendance.go` | HTTP handlers |
| `api/internal/handler/api/attendance_test.go` | Handler tests |

### Modified

| Path | Change |
|------|--------|
| `api/cmd/server/main.go` | Wire AttendanceService + handler; add to teacher route group |

---

## Tasks

### Task 1: `attendance` migration + RLS

`api/internal/migrate/migrations/20260512000002_attendance.go`:

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
			`CREATE TABLE attendance (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    halaqah_id UUID NOT NULL REFERENCES halaqat(id),
			    student_id UUID NOT NULL REFERENCES students(id),
			    date DATE NOT NULL,
			    status TEXT NOT NULL CHECK (status IN ('present', 'absent')),
			    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			    synced_at TIMESTAMPTZ,
			    client_id TEXT,
			    UNIQUE (student_id, date)
			)`,
			`CREATE INDEX idx_attendance_halaqah_date ON attendance(halaqah_id, date DESC)`,
			`CREATE INDEX idx_attendance_organization_id ON attendance(organization_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON attendance TO mutqin_app`,
			`ALTER TABLE attendance ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE attendance FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON attendance
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
			`DROP POLICY IF EXISTS tenant_isolation ON attendance`,
			`ALTER TABLE attendance NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE attendance DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE attendance`,
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

Commit `migrate: attendance table + RLS`.

---

### Task 2: `Attendance` model

```go
// api/internal/model/attendance.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Attendance struct {
	bun.BaseModel `bun:"table:attendance,alias:a"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	HalaqahID      uuid.UUID  `bun:"halaqah_id,notnull,type:uuid"`
	StudentID      uuid.UUID  `bun:"student_id,notnull,type:uuid"`
	Date           time.Time  `bun:"date,notnull,type:date"`
	Status         string     `bun:"status,notnull"`
	RecordedAt     time.Time  `bun:"recorded_at,notnull,nullzero,default:now()"`
	SyncedAt       *time.Time `bun:"synced_at"`
	ClientID       *string    `bun:"client_id"`
}
```

Commit `model: Attendance Bun model`.

---

### Task 3: `AttendanceRepo` (TDD)

Methods:
- `UpsertBatch(ctx, []Attendance) error` — `ON CONFLICT (student_id, date) DO UPDATE SET status = EXCLUDED.status, recorded_at = EXCLUDED.recorded_at` — idempotent.
- `ListByHalaqahDate(ctx, halaqahID, date) ([]Attendance, error)` — every row for a single day.
- `ListByStudent(ctx, studentID, from, to time.Time, limit, offset) ([]Attendance, error)` — DESC by date.
- `ListByOrg(ctx, halaqahID *uuid.UUID, from, to time.Time, limit, offset) ([]Attendance, error)` — optional halaqah filter (center admin's general view).

```go
// api/internal/repo/attendance.go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type AttendanceRepo struct {
	db bun.IDB
}

func NewAttendanceRepo(d bun.IDB) *AttendanceRepo {
	return &AttendanceRepo{db: d}
}

func (r *AttendanceRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

func (r *AttendanceRepo) UpsertBatch(ctx context.Context, rows []model.Attendance) error {
	if len(rows) == 0 {
		return nil
	}
	_, err := r.idb(ctx).NewInsert().
		Model(&rows).
		On("CONFLICT (student_id, date) DO UPDATE").
		Set("status = EXCLUDED.status").
		Set("recorded_at = EXCLUDED.recorded_at").
		Set("synced_at = EXCLUDED.synced_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("upsert attendance: %w", err)
	}
	return nil
}

func (r *AttendanceRepo) ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error) {
	var rows []model.Attendance
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("halaqah_id = ?", halaqahID).
		Where("date = ?", date.Format("2006-01-02")).
		OrderExpr("recorded_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by halaqah/date: %w", err)
	}
	return rows, nil
}

func (r *AttendanceRepo) ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	var rows []model.Attendance
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("student_id = ?", studentID).
		Where("date >= ?", from.Format("2006-01-02")).
		Where("date <= ?", to.Format("2006-01-02")).
		OrderExpr("date DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by student: %w", err)
	}
	return rows, nil
}

func (r *AttendanceRepo) ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	var rows []model.Attendance
	q := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("date >= ?", from.Format("2006-01-02")).
		Where("date <= ?", to.Format("2006-01-02"))
	if halaqahID != nil {
		q = q.Where("halaqah_id = ?", *halaqahID)
	}
	err := q.OrderExpr("date DESC").Limit(limit).Offset(offset).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list attendance by org: %w", err)
	}
	return rows, nil
}
```

Tests (4): UpsertBatch inserts N rows; UpsertBatch updates on conflict (same student+date, status flips); ListByHalaqahDate filters correctly; ListByStudent date range narrows.

Commit `repo: AttendanceRepo (UpsertBatch + 3 list queries)`.

---

### Task 4: `AttendanceService`

```go
// api/internal/service/attendance.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

var ErrInvalidAttendance = errors.New("service: invalid attendance input")

type attendanceRepoIface interface {
	UpsertBatch(ctx context.Context, rows []model.Attendance) error
	ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error)
	ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
	ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
}

type AttendanceService struct {
	att   attendanceRepoIface
	audit auditRepo
}

func NewAttendanceService(att attendanceRepoIface, audit auditRepo) *AttendanceService {
	return &AttendanceService{att: att, audit: audit}
}

// MarkInput represents one student's attendance for a date.
type MarkInput struct {
	StudentID uuid.UUID
	Status    string // "present" | "absent"
	ClientID  *string
}

// MarkBatch upserts attendance for a halaqah on a given date. Idempotent —
// re-running with different statuses overrides the previous values.
func (s *AttendanceService) MarkBatch(ctx context.Context, actorID, orgID, halaqahID uuid.UUID, date time.Time, marks []MarkInput) ([]model.Attendance, error) {
	if len(marks) == 0 {
		return nil, fmt.Errorf("%w: empty marks", ErrInvalidAttendance)
	}
	rows := make([]model.Attendance, 0, len(marks))
	for i, m := range marks {
		if m.StudentID == uuid.Nil {
			return nil, fmt.Errorf("%w: mark %d: student_id required", ErrInvalidAttendance, i)
		}
		if m.Status != "present" && m.Status != "absent" {
			return nil, fmt.Errorf("%w: mark %d: status must be present or absent", ErrInvalidAttendance, i)
		}
		rows = append(rows, model.Attendance{
			OrganizationID: orgID,
			HalaqahID:      halaqahID,
			StudentID:      m.StudentID,
			Date:           date,
			Status:         m.Status,
			ClientID:       m.ClientID,
		})
	}
	if err := s.att.UpsertBatch(ctx, rows); err != nil {
		return nil, fmt.Errorf("upsert: %w", err)
	}

	tt := "attendance"
	tid := halaqahID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "mark_attendance_batch",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return rows, nil
}

func (s *AttendanceService) ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error) {
	return s.att.ListByHalaqahDate(ctx, halaqahID, date)
}

func (s *AttendanceService) ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.att.ListByStudent(ctx, studentID, from, to, limit, offset)
}

func (s *AttendanceService) ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.att.ListByOrg(ctx, halaqahID, from, to, limit, offset)
}
```

Tests: MarkBatch happy path + audit; MarkBatch empty → ErrInvalidAttendance; MarkBatch invalid status → ErrInvalidAttendance; ListByStudent clamps; ListByOrg with/without halaqah filter.

Commit `service: AttendanceService (MarkBatch + list queries + audit)`.

---

### Task 5: HTTP handler

Endpoints:
- `POST /api/v1/halaqat/{id}/attendance` — body `{date: "YYYY-MM-DD", marks: [{student_id, status, client_id?}, ...]}` — returns 200 with the upserted rows.
- `GET /api/v1/halaqat/{id}/attendance?date=YYYY-MM-DD` — single-day list for the halaqah.
- `GET /api/v1/students/{id}/attendance?from=&to=&limit=&offset=` — student history.
- `GET /api/v1/attendance?halaqah_id=&date_from=&date_to=&limit=&offset=` — org-wide query, optional halaqah filter.

Validation:
- Date format `YYYY-MM-DD` — use `time.Parse("2006-01-02", ...)`. Bad format → 400.
- `from`/`to` default to today / 30-days-ago if absent.
- Map `service.ErrInvalidAttendance` → 400 with detail.

Tests (5+): MarkBatch happy path → 200 with rows; MarkBatch invalid status → 400; ListByHalaqahDate happy + bad date → 400; ListByStudent + ListByOrg pass-through.

Commit `handler: attendance (mark + list queries)`.

---

### Task 6: Wire `cmd/server/main.go`

After existing service wiring, add:

```go
	attendanceSvc := service.NewAttendanceService(repo.NewAttendanceRepo(appDB), auditRepo)
	attendanceH := apihandler.NewAttendanceHandler(attendanceSvc)
```

Routes — `POST /halaqat/{id}/attendance` and `GET /halaqat/{id}/attendance` go in the teacher route group (teacher records + reads same-day); `GET /students/{id}/attendance` and `GET /attendance` go in the center_admin group (history/cross-halaqah is admin scope).

```go
	r.Group(func(tr chi.Router) {
		tr.Use(middleware.Role("teacher", "center_admin", "super_admin"))
		// ... existing routes ...
		tr.Post("/api/v1/halaqat/{id}/attendance", attendanceH.MarkBatch)
		tr.Get("/api/v1/halaqat/{id}/attendance", attendanceH.ListByHalaqahDate)
	})

	r.Group(func(ca chi.Router) {
		ca.Use(middleware.Role("center_admin", "super_admin"))
		// ... existing routes ...
		ca.Get("/api/v1/students/{id}/attendance", attendanceH.ListByStudent)
		ca.Get("/api/v1/attendance", attendanceH.ListByOrg)
	})
```

Commit `cmd: wire AttendanceService + teacher mark + admin query routes`.

---

### Task 7: End-to-end smoke

Bootstrap → org → CA → halaqah → 3 students → teacher invite/accept. As teacher:
1. POST mark batch (all 3 present)
2. GET same-day attendance → 3 rows present
3. POST mark batch again (1 absent now)
4. GET → status flipped via upsert (still 3 rows; one now absent)
5. As CA: GET /students/{stuId}/attendance?from=&to= → see all rows

Verify DB UNIQUE (student_id, date) enforced.

---

### Task 8: Final verify + push

Tests + vet + push to origin + gitlab. PR + MR.

---

## Self-Review

**Coverage:**
- FR31 (mark present/absent) ✓
- FR32 (bulk-mark, idempotent re-runs) ✓ via UpsertBatch
- FR33 (offline) — Plan N hooks into UpsertBatch through `client_id` dedupe
- FR34 (CA views by halaqah/student/date) ✓ — 3 list endpoints

**Type / signature consistency:**
- `service.MarkInput{StudentID, Status, ClientID}` matches handler body.
- Date as `time.Time` everywhere; formatted to `YYYY-MM-DD` in SQL bind.
- `service.ErrInvalidAttendance` → 400 via `errors.Is`.

**Scope:** 8 tasks. Substantive: 1, 3, 4, 5. Trivial: 2. Wire/smoke/push: 6, 7, 8.
