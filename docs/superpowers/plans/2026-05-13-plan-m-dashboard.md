# Plan M — Dashboard + Reporting

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement task-by-task.

**Goal:** Center-admin dashboard (counts, attendance rate, recitation activity) plus super-admin platform stats and audit-log view.

**Architecture:** Read-only aggregate queries via a new `DashboardRepo` and `PlatformStatsRepo` (both raw SQL — `COUNT`, `AVG`, grouped aggregates — easier than Bun's relational builder for these shapes). `DashboardService` (tenant-scoped, RLS-aware via `idb(ctx)`) and `PlatformService` (admin handle, cross-tenant). Five new handler endpoints, gated by existing route groups.

**Tech Stack:** existing. New file pattern: `repo/dashboard.go` uses `bun.IDB.QueryContext` with raw SQL + `Scan` into a result struct.

**Out of scope:**
- Charts / time-series graphs (frontend)
- Cached / pre-computed aggregates (materialized views) — premature
- Export to CSV — separate plan

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/repo/dashboard.go` | `DashboardRepo` — center-scoped aggregates over halaqat/students/attendance/recitations |
| `api/internal/repo/dashboard_test.go` | Integration |
| `api/internal/repo/platform_stats.go` | `PlatformStatsRepo` — cross-tenant aggregates (super_admin) |
| `api/internal/repo/platform_stats_test.go` | Integration |
| `api/internal/service/dashboard.go` | `DashboardService` + `PlatformService` |
| `api/internal/service/dashboard_test.go` | Mocked tests |
| `api/internal/handler/api/dashboard.go` | 3 center-admin endpoints |
| `api/internal/handler/api/dashboard_test.go` | Handler tests |
| `api/internal/handler/api/platform_stats.go` | 2 super-admin endpoints |
| `api/internal/handler/api/platform_stats_test.go` | Handler tests |

### Modified

| Path | Change |
|------|--------|
| `api/cmd/server/main.go` | Wire dashboard + platform-stats services + handlers |

---

## Endpoints

### Center-admin (`Role("center_admin", "super_admin")`)

| Path | Returns |
|------|---------|
| `GET /api/v1/dashboard/stats` | `{ students, halaqat, teachers, recitations_30d, attendance_rate_30d }` |
| `GET /api/v1/dashboard/attendance-trends?from=&to=&halaqah_id=` | `[{ date, present_count, absent_count, total }, ...]` |
| `GET /api/v1/dashboard/recitation-activity?from=&to=` | `[{ halaqah_id, halaqah_name, count }, ...]` |

### Super-admin (`Role("super_admin")`)

| Path | Returns |
|------|---------|
| `GET /api/v1/platform/stats` | `{ centers, active_centers, suspended_centers, total_students, total_recitations_30d }` |
| `GET /api/v1/platform/audit-log?actor_id=&limit=&offset=` | Wrapped list of `audit_log` rows |

---

## Tasks

### Task 1: `DashboardRepo` (TDD)

`api/internal/repo/dashboard.go` — uses `r.idb(ctx)` so queries run under RLS context.

```go
package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
)

type DashboardRepo struct {
	db bun.IDB
}

func NewDashboardRepo(d bun.IDB) *DashboardRepo {
	return &DashboardRepo{db: d}
}

func (r *DashboardRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

type CenterStats struct {
	Students           int     `json:"students"`
	Halaqat            int     `json:"halaqat"`
	Teachers           int     `json:"teachers"`
	Recitations30d     int     `json:"recitations_30d"`
	AttendanceRate30d  float64 `json:"attendance_rate_30d"` // 0-1
}

// CenterStats computes counts for the current tenant. All tables are RLS-subject
// (organization_id filter is enforced by Postgres + per-model hooks).
func (r *DashboardRepo) CenterStats(ctx context.Context) (*CenterStats, error) {
	stats := &CenterStats{}
	idb := r.idb(ctx)

	row := idb.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM students  WHERE status = 'active') AS students,
		  (SELECT COUNT(*) FROM halaqat   WHERE status = 'active') AS halaqat,
		  (SELECT COUNT(*) FROM users     WHERE role = 'teacher' AND status = 'active') AS teachers,
		  (SELECT COUNT(*) FROM recitations WHERE recorded_at >= now() - interval '30 days') AS recitations_30d,
		  COALESCE((
		    SELECT AVG(CASE WHEN status='present' THEN 1.0 ELSE 0.0 END)
		    FROM attendance
		    WHERE date >= (now() - interval '30 days')::date
		  ), 0) AS attendance_rate_30d
	`)
	if err := row.Scan(&stats.Students, &stats.Halaqat, &stats.Teachers, &stats.Recitations30d, &stats.AttendanceRate30d); err != nil {
		return nil, fmt.Errorf("center stats: %w", err)
	}
	return stats, nil
}

type AttendanceTrendRow struct {
	Date         time.Time `json:"date"`
	PresentCount int       `json:"present_count"`
	AbsentCount  int       `json:"absent_count"`
	Total        int       `json:"total"`
}

// AttendanceTrends returns one row per date in [from, to]. Optional halaqahID.
func (r *DashboardRepo) AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]AttendanceTrendRow, error) {
	rows := make([]AttendanceTrendRow, 0)
	idb := r.idb(ctx)

	query := `
		SELECT date,
		       SUM(CASE WHEN status='present' THEN 1 ELSE 0 END) AS present,
		       SUM(CASE WHEN status='absent'  THEN 1 ELSE 0 END) AS absent,
		       COUNT(*) AS total
		FROM attendance
		WHERE date >= $1 AND date <= $2
	`
	args := []any{from.Format("2006-01-02"), to.Format("2006-01-02")}
	if halaqahID != nil {
		query += " AND halaqah_id = $3"
		args = append(args, *halaqahID)
	}
	query += " GROUP BY date ORDER BY date"

	r2, err := idb.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("attendance trends: %w", err)
	}
	defer r2.Close()
	for r2.Next() {
		var row AttendanceTrendRow
		if err := r2.Scan(&row.Date, &row.PresentCount, &row.AbsentCount, &row.Total); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

type RecitationActivityRow struct {
	HalaqahID   uuid.UUID `json:"halaqah_id"`
	HalaqahName string    `json:"halaqah_name"`
	Count       int       `json:"count"`
}

func (r *DashboardRepo) RecitationActivity(ctx context.Context, from, to time.Time) ([]RecitationActivityRow, error) {
	rows := make([]RecitationActivityRow, 0)
	r2, err := r.idb(ctx).QueryContext(ctx, `
		SELECT h.id, h.name, COUNT(rec.id) AS cnt
		FROM halaqat h
		LEFT JOIN recitations rec
		  ON rec.halaqah_id = h.id
		 AND rec.recorded_at >= $1
		 AND rec.recorded_at <= $2
		GROUP BY h.id, h.name
		ORDER BY cnt DESC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("recitation activity: %w", err)
	}
	defer r2.Close()
	for r2.Next() {
		var row RecitationActivityRow
		if err := r2.Scan(&row.HalaqahID, &row.HalaqahName, &row.Count); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
```

Tests (3): CenterStats counts correctly with seeded data; AttendanceTrends groups by date; RecitationActivity orders by count DESC.

Commit `repo: DashboardRepo (CenterStats, AttendanceTrends, RecitationActivity)`.

---

### Task 2: `PlatformStatsRepo` (TDD)

Cross-tenant aggregates — uses raw admin handle (no RLS / no tenant ctx); explicit `WHERE` if needed.

```go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type PlatformStatsRepo struct {
	db bun.IDB
}

func NewPlatformStatsRepo(d bun.IDB) *PlatformStatsRepo {
	return &PlatformStatsRepo{db: d}
}

type PlatformStats struct {
	Centers              int `json:"centers"`
	ActiveCenters        int `json:"active_centers"`
	SuspendedCenters     int `json:"suspended_centers"`
	TotalStudents        int `json:"total_students"`
	TotalRecitations30d  int `json:"total_recitations_30d"`
}

func (r *PlatformStatsRepo) Compute(ctx context.Context) (*PlatformStats, error) {
	s := &PlatformStats{}
	row := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM organizations) AS centers,
		  (SELECT COUNT(*) FROM organizations WHERE status='active') AS active,
		  (SELECT COUNT(*) FROM organizations WHERE status='suspended') AS suspended,
		  (SELECT COUNT(*) FROM students WHERE status='active') AS students,
		  (SELECT COUNT(*) FROM recitations WHERE recorded_at >= now() - interval '30 days') AS rec30d
	`)
	if err := row.Scan(&s.Centers, &s.ActiveCenters, &s.SuspendedCenters, &s.TotalStudents, &s.TotalRecitations30d); err != nil {
		return nil, fmt.Errorf("platform stats: %w", err)
	}
	return s, nil
}
```

> **Note:** PlatformStatsRepo's queries touch `students` and `recitations` (RLS-subject), but the repo is constructed with the **admin handle** (mutqin user, superuser, bypasses RLS). This is intentional — super-admin sees cross-tenant aggregate.

Tests (1+): seed 2 orgs with students + recitations; Compute returns correct totals.

Commit `repo: PlatformStatsRepo (cross-tenant aggregates)`.

---

### Task 3: `DashboardService` + `PlatformService` (mocked tests)

`api/internal/service/dashboard.go`:

```go
package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/repo"
)

type dashboardRepoIface interface {
	CenterStats(ctx context.Context) (*repo.CenterStats, error)
	AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error)
	RecitationActivity(ctx context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error)
}

type DashboardService struct {
	repo dashboardRepoIface
}

func NewDashboardService(r dashboardRepoIface) *DashboardService {
	return &DashboardService{repo: r}
}

func (s *DashboardService) CenterStats(ctx context.Context) (*repo.CenterStats, error) {
	return s.repo.CenterStats(ctx)
}

func (s *DashboardService) AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error) {
	return s.repo.AttendanceTrends(ctx, halaqahID, from, to)
}

func (s *DashboardService) RecitationActivity(ctx context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error) {
	return s.repo.RecitationActivity(ctx, from, to)
}

// PlatformService is super-admin only.

type platformRepoIface interface {
	Compute(ctx context.Context) (*repo.PlatformStats, error)
}

type auditLister interface {
	ListByActor(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error)
}

type PlatformService struct {
	stats platformRepoIface
	audit auditLister
}

func NewPlatformService(s platformRepoIface, a auditLister) *PlatformService {
	return &PlatformService{stats: s, audit: a}
}

func (s *PlatformService) Stats(ctx context.Context) (*repo.PlatformStats, error) {
	return s.stats.Compute(ctx)
}

func (s *PlatformService) AuditLog(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.audit.ListByActor(ctx, actorID, limit, offset)
}
```

Add `import "github.com/ilyas/mutqin-api/internal/model"` to support the AuditLog type.

Stub tests: each service pass-through clamps + delegates.

Commit `service: DashboardService + PlatformService`.

---

### Task 4: Dashboard handler (TDD)

`api/internal/handler/api/dashboard.go`:
- `DashboardService` interface (3 methods)
- `DashboardHandler` with `Stats`, `AttendanceTrends`, `RecitationActivity`
- Parse `from`/`to` from query, default to today-30d / today. Bad date → 400.
- Optional `halaqah_id` query for `AttendanceTrends` (UUID, 400 if malformed)
- Wrap response in `{"data": ...}`. Lists use `response.SuccessList(w, rows, len(rows))`.

Tests: 4+ (stats happy path, trends with halaqah filter, recitation-activity default range, bad date → 400).

Commit `handler: dashboard (stats, attendance trends, recitation activity)`.

---

### Task 5: Platform-stats handler (TDD)

`api/internal/handler/api/platform_stats.go`:
- `PlatformStatsService` interface (2 methods)
- `PlatformStatsHandler` with `Stats`, `AuditLog`
- `Stats`: no params, returns `PlatformStats`.
- `AuditLog`: query params `actor_id` (UUID, optional — if absent return empty list with note), `limit`, `offset`.

Tests: 3+ (Stats happy path, AuditLog with actor filter, missing actor → returns empty list with 200).

Commit `handler: platform stats + audit log`.

---

### Task 6: Wire `cmd/server/main.go`

```go
dashboardSvc := service.NewDashboardService(repo.NewDashboardRepo(appDB))
platformStatsSvc := service.NewPlatformService(repo.NewPlatformStatsRepo(adminDB), auditRepo)
dashboardH := apihandler.NewDashboardHandler(dashboardSvc)
platformStatsH := apihandler.NewPlatformStatsHandler(platformStatsSvc)
```

Add to existing groups:

```go
// In super_admin group:
pr.Get("/api/v1/platform/stats", platformStatsH.Stats)
pr.Get("/api/v1/platform/audit-log", platformStatsH.AuditLog)

// In center_admin group:
ca.Get("/api/v1/dashboard/stats", dashboardH.Stats)
ca.Get("/api/v1/dashboard/attendance-trends", dashboardH.AttendanceTrends)
ca.Get("/api/v1/dashboard/recitation-activity", dashboardH.RecitationActivity)
```

Commit `cmd: wire DashboardService + PlatformService + 5 read endpoints`.

---

### Task 7: Final verify + push

Tests + vet + push to origin + gitlab. No docker smoke (controller can do later).

---

## Self-Review

**PRD coverage:**
- FR3 (platform-wide dashboard) ✓ Task 5
- FR35 (center stats: students/halaqat/teachers/attendance rate) ✓ Task 1, 4
- FR36 (attendance trends) ✓
- FR37 (recitation activity) ✓
- FR38-39 (audit log view) — basic; richer filtering deferred

**Type / signature consistency:**
- Repo returns concrete result structs from `repo` package; service + handler import them.
- Dashboard repo: tenant-scoped via `r.idb(ctx)`. PlatformStats repo: admin handle, no tenant.

**Scope:** 7 tasks. Substantive: 1, 2, 4, 5. Trivial: 3 (passthrough services). Wire/push: 6, 7.
