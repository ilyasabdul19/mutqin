# Plan K — Recitation Recording

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Teachers record student recitation sessions (single + batch), view history, and see the last position per student. RLS-subject tenant table with full backend support.

**Architecture:** A new `recitations` table — RLS-subject, scoped to organization, indexed by student_id + recorded_at — backs all queries. `RecitationRepo` follows the established `r.idb(ctx)` pattern so RLS context fires. `RecitationService` validates input (surah 1-114, ayah ordering, enum types/grades) and audits. Handlers expose single + batch record paths so offline-collected sessions can be flushed in one round trip (Plan N hooks here).

**Tech Stack:** existing — Bun, Chi, JWT, RLS, established service/handler patterns.

**Out of scope (deferred):**
- Quran reference data seeding (114 surahs / 6236 ayat → static JSON in `web/public/quran/`) — this is a frontend asset task, separate plan
- Suggest-next-ayah algorithm (FR28) — needs Quran data; defer to a small follow-up plan once reference data lands
- Offline-sync mechanics (FR30) — Plan N
- Per-surah ayah-count validation (`surah X has 7 ayat, can't ayah_to=10`) — needs reference data; basic ordering validated for now
- Restrict teachers to only their assigned halaqah (within-tenant teacher-vs-teacher access) — RLS handles cross-tenant; intra-tenant teacher scoping is a small follow-up

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/migrate/migrations/20260512000001_recitations.go` | `recitations` table + RLS policy + grants |
| `api/internal/model/recitation.go` | Bun model (embeds `TenantScoped`) |
| `api/internal/repo/recitation.go` | `RecitationRepo` (Create, BatchCreate, ListByStudent, GetLatestByStudent) — uses `r.idb(ctx)` |
| `api/internal/repo/recitation_test.go` | Integration |
| `api/internal/service/recitation.go` | Validation + audit + orchestration |
| `api/internal/service/recitation_test.go` | Mocked tests |
| `api/internal/handler/api/recitations.go` | HTTP handlers |
| `api/internal/handler/api/recitations_test.go` | Handler tests |

### Modified

| Path | Change |
|------|--------|
| `api/cmd/server/main.go` | Wire RecitationService + handler; add to teacher route group |

### Deleted
None.

---

## Tasks

### Task 1: `recitations` migration + RLS

**Files:**
- Create: `api/internal/migrate/migrations/20260512000001_recitations.go`

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
			`CREATE TABLE recitations (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    student_id UUID NOT NULL REFERENCES students(id),
			    halaqah_id UUID NOT NULL REFERENCES halaqat(id),
			    teacher_id UUID NOT NULL REFERENCES users(id),
			    type TEXT NOT NULL CHECK (type IN ('new_hifz', 'near_review', 'far_review')),
			    surah_number INT NOT NULL CHECK (surah_number BETWEEN 1 AND 114),
			    ayah_from INT NOT NULL CHECK (ayah_from >= 1),
			    ayah_to INT NOT NULL CHECK (ayah_to >= ayah_from),
			    grade TEXT NOT NULL CHECK (grade IN ('mumtaz', 'jayyid_jiddan', 'jayyid', 'maqbul', 'daif')),
			    notes TEXT,
			    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			    synced_at TIMESTAMPTZ,
			    client_id TEXT
			)`,
			`CREATE INDEX idx_recitations_student_recorded
			   ON recitations(student_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_halaqah_recorded
			   ON recitations(halaqah_id, recorded_at DESC)`,
			`CREATE INDEX idx_recitations_organization_id
			   ON recitations(organization_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON recitations TO mutqin_app`,
			`ALTER TABLE recitations ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE recitations FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON recitations
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
			`DROP POLICY IF EXISTS tenant_isolation ON recitations`,
			`ALTER TABLE recitations NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE recitations DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE recitations`,
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

Commit `migrate: recitations table + RLS`.

---

### Task 2: `Recitation` model

**Files:**
- Create: `api/internal/model/recitation.go`

```go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Recitation struct {
	bun.BaseModel `bun:"table:recitations,alias:r"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	StudentID      uuid.UUID  `bun:"student_id,notnull,type:uuid"`
	HalaqahID      uuid.UUID  `bun:"halaqah_id,notnull,type:uuid"`
	TeacherID      uuid.UUID  `bun:"teacher_id,notnull,type:uuid"`
	Type           string     `bun:"type,notnull"`
	SurahNumber    int        `bun:"surah_number,notnull"`
	AyahFrom       int        `bun:"ayah_from,notnull"`
	AyahTo         int        `bun:"ayah_to,notnull"`
	Grade          string     `bun:"grade,notnull"`
	Notes          *string    `bun:"notes"`
	RecordedAt     time.Time  `bun:"recorded_at,notnull,nullzero,default:now()"`
	SyncedAt       *time.Time `bun:"synced_at"`
	ClientID       *string    `bun:"client_id"`
}
```

Commit `model: Recitation Bun model`.

---

### Task 3: `RecitationRepo` (TDD)

**Files:**
- Create: `api/internal/repo/recitation.go`
- Create: `api/internal/repo/recitation_test.go`

Methods:
- `Create(ctx, *Recitation) error`
- `BatchCreate(ctx, []model.Recitation) error` — bulk insert in one round-trip
- `ListByStudent(ctx, studentID, limit, offset) ([]Recitation, error)` — DESC by recorded_at
- `GetLatestByStudent(ctx, studentID) (*Recitation, error)` — single, latest

All methods use `r.idb(ctx)` (same pattern as HalaqahRepo).

Tests (3+): Create + GetLatest returns it; ListByStudent returns DESC ordering; BatchCreate inserts N rows in one call.

Implementation:

```go
// api/internal/repo/recitation.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type RecitationRepo struct {
	db bun.IDB
}

func NewRecitationRepo(d bun.IDB) *RecitationRepo {
	return &RecitationRepo{db: d}
}

func (r *RecitationRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

func (r *RecitationRepo) Create(ctx context.Context, rec *model.Recitation) error {
	_, err := r.idb(ctx).NewInsert().Model(rec).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert recitation: %w", err)
	}
	return nil
}

func (r *RecitationRepo) BatchCreate(ctx context.Context, recs []model.Recitation) error {
	if len(recs) == 0 {
		return nil
	}
	_, err := r.idb(ctx).NewInsert().Model(&recs).Exec(ctx)
	if err != nil {
		return fmt.Errorf("batch insert recitations: %w", err)
	}
	return nil
}

func (r *RecitationRepo) ListByStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	var rows []model.Recitation
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("student_id = ?", studentID).
		OrderExpr("recorded_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recitations by student: %w", err)
	}
	return rows, nil
}

func (r *RecitationRepo) GetLatestByStudent(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	rec := new(model.Recitation)
	err := r.idb(ctx).NewSelect().
		Model(rec).
		Where("student_id = ?", studentID).
		OrderExpr("recorded_at DESC").
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest recitation: %w", err)
	}
	return rec, nil
}
```

Tests follow the established pattern (use `testAdmin` for setup, `tenant.With(ctx, orgID)` for scoped reads). Commit `repo: RecitationRepo (Create, BatchCreate, ListByStudent, GetLatestByStudent)`.

---

### Task 4: `RecitationService` (TDD with mocks)

**Files:**
- Create: `api/internal/service/recitation.go`
- Create: `api/internal/service/recitation_test.go`

```go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

var (
	ErrInvalidRecitationInput = errors.New("service: invalid recitation input")
)

var validTypes = map[string]struct{}{
	"new_hifz": {}, "near_review": {}, "far_review": {},
}
var validGrades = map[string]struct{}{
	"mumtaz": {}, "jayyid_jiddan": {}, "jayyid": {}, "maqbul": {}, "daif": {},
}

type recitationRepoIface interface {
	Create(ctx context.Context, r *model.Recitation) error
	BatchCreate(ctx context.Context, rs []model.Recitation) error
	ListByStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error)
	GetLatestByStudent(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error)
}

type RecitationService struct {
	recs  recitationRepoIface
	audit auditRepo
}

func NewRecitationService(recs recitationRepoIface, audit auditRepo) *RecitationService {
	return &RecitationService{recs: recs, audit: audit}
}

// RecordInput is the validated payload for a single recitation. ClientID is
// optional and used by offline sync (Plan N) to dedupe.
type RecordInput struct {
	StudentID   uuid.UUID
	HalaqahID   uuid.UUID
	Type        string
	SurahNumber int
	AyahFrom    int
	AyahTo      int
	Grade       string
	Notes       *string
	ClientID    *string
}

func validateInput(in RecordInput) error {
	if in.StudentID == uuid.Nil || in.HalaqahID == uuid.Nil {
		return fmt.Errorf("%w: student_id and halaqah_id required", ErrInvalidRecitationInput)
	}
	if _, ok := validTypes[in.Type]; !ok {
		return fmt.Errorf("%w: type must be new_hifz/near_review/far_review", ErrInvalidRecitationInput)
	}
	if _, ok := validGrades[in.Grade]; !ok {
		return fmt.Errorf("%w: grade must be mumtaz/jayyid_jiddan/jayyid/maqbul/daif", ErrInvalidRecitationInput)
	}
	if in.SurahNumber < 1 || in.SurahNumber > 114 {
		return fmt.Errorf("%w: surah_number must be 1..114", ErrInvalidRecitationInput)
	}
	if in.AyahFrom < 1 || in.AyahTo < in.AyahFrom {
		return fmt.Errorf("%w: ayah range invalid", ErrInvalidRecitationInput)
	}
	return nil
}

func (s *RecitationService) Record(ctx context.Context, actorID, orgID uuid.UUID, in RecordInput) (*model.Recitation, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	rec := &model.Recitation{
		OrganizationID: orgID,
		StudentID:      in.StudentID,
		HalaqahID:      in.HalaqahID,
		TeacherID:      actorID,
		Type:           in.Type,
		SurahNumber:    in.SurahNumber,
		AyahFrom:       in.AyahFrom,
		AyahTo:         in.AyahTo,
		Grade:          in.Grade,
		Notes:          in.Notes,
		ClientID:       in.ClientID,
	}
	if err := s.recs.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("record recitation: %w", err)
	}

	tt := "recitation"
	tid := rec.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "record_recitation",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return rec, nil
}

func (s *RecitationService) RecordBatch(ctx context.Context, actorID, orgID uuid.UUID, ins []RecordInput) ([]model.Recitation, error) {
	if len(ins) == 0 {
		return nil, nil
	}
	recs := make([]model.Recitation, 0, len(ins))
	for i, in := range ins {
		if err := validateInput(in); err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		recs = append(recs, model.Recitation{
			OrganizationID: orgID,
			StudentID:      in.StudentID,
			HalaqahID:      in.HalaqahID,
			TeacherID:      actorID,
			Type:           in.Type,
			SurahNumber:    in.SurahNumber,
			AyahFrom:       in.AyahFrom,
			AyahTo:         in.AyahTo,
			Grade:          in.Grade,
			Notes:          in.Notes,
			ClientID:       in.ClientID,
		})
	}
	if err := s.recs.BatchCreate(ctx, recs); err != nil {
		return nil, fmt.Errorf("batch record: %w", err)
	}

	tt := "recitation"
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "record_recitation_batch",
		TargetType: &tt,
	})
	return recs, nil
}

func (s *RecitationService) ListForStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.recs.ListByStudent(ctx, studentID, limit, offset)
}

func (s *RecitationService) GetLatest(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	return s.recs.GetLatestByStudent(ctx, studentID)
}
```

Tests (~6 with mocks): Record happy path + audit; Record validation rejects bad type/grade/surah/ayah; RecordBatch validates each input; ListForStudent clamps limit; GetLatest passthrough.

Commit `service: RecitationService (Record, RecordBatch, ListForStudent, GetLatest + validation + audit)`.

---

### Task 5: HTTP handler (TDD)

**Files:**
- Create: `api/internal/handler/api/recitations.go`
- Create: `api/internal/handler/api/recitations_test.go`

Endpoints:
- `POST /api/v1/recitations` — single record. Body: `{student_id, halaqah_id, type, surah_number, ayah_from, ayah_to, grade, notes?, client_id?}`. Returns 201 with the created recitation.
- `POST /api/v1/recitations/batch` — array body. Returns 201 with count + ids.
- `GET /api/v1/students/{id}/recitations` — query `limit`, `offset`. Wrapped list.
- `GET /api/v1/students/{id}/recitations/latest` — single. 404 on none.

Map `service.ErrInvalidRecitationInput` → 400 with message detail. Other errors → 500. RBAC: enforced by route group at wire time (teacher, center_admin, super_admin).

Commit `handler: recitations (record, batch, list, latest)`.

---

### Task 6: Wire `cmd/server/main.go`

**Files:**
- Modify: `api/cmd/server/main.go`

After Plan J's wiring, add:

```go
	recitationSvc := service.NewRecitationService(repo.NewRecitationRepo(appDB), auditRepo)
	recitationsH := apihandler.NewRecitationsHandler(recitationSvc)
```

Add routes to the teacher group (which already includes center_admin + super_admin):

```go
	r.Group(func(tr chi.Router) {
		tr.Use(middleware.Role("teacher", "center_admin", "super_admin"))
		tr.Get("/api/v1/halaqat/{id}/students", studentsH.ListByHalaqah)
		tr.Post("/api/v1/recitations", recitationsH.Record)
		tr.Post("/api/v1/recitations/batch", recitationsH.RecordBatch)
		tr.Get("/api/v1/students/{id}/recitations", recitationsH.ListByStudent)
		tr.Get("/api/v1/students/{id}/recitations/latest", recitationsH.LatestByStudent)
	})
```

Commit `cmd: wire RecitationService + teacher recitation routes`.

---

### Task 7: End-to-end smoke

Bring up stack. Bootstrap super_admin → org → center_admin → halaqah → enroll students → teacher invite + accept. As teacher: POST a recitation. GET latest. GET list. POST batch of 3. Verify DB row count = 4 + recorded_at descending.

---

### Task 8: Final verify + push

Tests + vet + push to origin + gitlab. PR + MR with summary.

---

## Self-Review

**Coverage:**
- FR26 (record recitation with type/surah/ayah/grade/notes) ✓
- FR29 (batch record for session flow) ✓ via /recitations/batch
- FR25 (view student list) — already in Plan J
- FR27 (last position) ✓ GET /recitations/latest
- FR28 (suggest next) — deferred; needs Quran reference data
- FR30 (offline sync) — Plan N

**Type / signature consistency:**
- `service.RecordInput` fields match handler JSON body fields.
- `RecitationRepo.idb(ctx)` pattern matches HalaqahRepo / StudentRepo.
- Audit actions: `record_recitation`, `record_recitation_batch`.

**Scope:** 8 tasks. Substantive: 1, 3, 4, 5. Trivial: 2. Wire/smoke/push: 6, 7, 8.
