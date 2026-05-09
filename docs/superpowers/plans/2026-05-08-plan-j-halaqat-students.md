# Plan J — Halaqat + Teachers + Students

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Center admins manage halaqat (study circles), invite teachers, and enroll/transfer students. Teachers see only their assigned halaqah's students.

**Architecture:** Two new tenant-scoped tables (`halaqat`, `students`) with RLS, embeddable `TenantScoped` models, full CRUD repos, and per-feature services. `InviteService.GenerateForTeacher` reuses the existing invite-accept flow (the `center_admin` path from Plan I generalizes — only the role string differs). A new `AuthAsTenant` middleware bridges `auth.Identity.OrgID` into `tenant.With` so RLS context fires for authenticated requests with no Host-based slug. Center-admin and teacher routes are gated by `Role(...)`.

**Tech Stack:** existing — Bun, Chi, RLS, JWT, OTP, the established service layer pattern.

**Out of scope (deferred):**
- Recitation tracking on student progress (Plan K)
- Attendance for halaqat (Plan L)
- Center-admin dashboard with student counts and trends (Plan M)
- Bulk student import (CSV upload) — separate small follow-up

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/migrate/migrations/20260508000003_halaqat.go` | `halaqat` table + RLS policy + `mutqin_app` grants |
| `api/internal/migrate/migrations/20260508000004_students.go` | `students` table + RLS policy + grants |
| `api/internal/model/halaqah.go` | Bun model (embeds `TenantScoped`) |
| `api/internal/model/student.go` | Bun model (embeds `TenantScoped`) |
| `api/internal/repo/halaqah.go` | `HalaqahRepo` — Create / GetByID / ListByOrg / Update |
| `api/internal/repo/halaqah_test.go` | Integration |
| `api/internal/repo/student.go` | `StudentRepo` — Create / GetByID / ListByHalaqah / ListByOrg / Update / Transfer |
| `api/internal/repo/student_test.go` | Integration |
| `api/internal/service/halaqah.go` | Business rules, audit |
| `api/internal/service/halaqah_test.go` | Mocked tests |
| `api/internal/service/student.go` | Enroll, transfer, list-by-halaqah |
| `api/internal/service/student_test.go` | Mocked tests |
| `api/internal/handler/api/halaqat.go` | CRUD endpoints |
| `api/internal/handler/api/halaqat_test.go` | Handler tests |
| `api/internal/handler/api/students.go` | Enroll / list / transfer endpoints |
| `api/internal/handler/api/students_test.go` | Handler tests |
| `api/internal/handler/api/teachers.go` | Generate teacher invite endpoint |
| `api/internal/handler/api/teachers_test.go` | Handler tests |
| `api/internal/middleware/auth_as_tenant.go` | Bridges `auth.Identity.OrgID` into `tenant.With` |
| `api/internal/middleware/auth_as_tenant_test.go` | Tests |

### Modified

| Path | Change |
|------|--------|
| `api/internal/service/invite.go` | Add `GenerateForTeacher(ctx, actor, orgID) (*Invite, error)` — same shape as the existing `GenerateForCenterAdmin`, role string differs |
| `api/internal/service/invite_test.go` | Test for the new method |
| `api/internal/handler/api/platform.go` | Stays — but the `InviteIssuer` interface might need a second method; alternative: a new `TeacherInviteIssuer` on the new teachers handler |
| `api/cmd/server/main.go` | Wire new services + handlers; add `AuthAsTenant` to middleware chain; add center-admin and teacher route groups |

### Deleted
None.

---

## Tasks

### Task 1: `halaqat` migration + RLS

**Files:**
- Create: `api/internal/migrate/migrations/20260508000003_halaqat.go`

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
			`CREATE TABLE halaqat (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    name TEXT NOT NULL,
			    teacher_id UUID REFERENCES users(id),
			    schedule JSONB,
			    max_capacity INT NOT NULL DEFAULT 30,
			    status TEXT NOT NULL DEFAULT 'active'
			      CHECK (status IN ('active', 'inactive')),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_halaqat_organization_id ON halaqat(organization_id)`,
			`CREATE INDEX idx_halaqat_teacher_id ON halaqat(teacher_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON halaqat TO mutqin_app`,
			`ALTER TABLE halaqat ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE halaqat FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON halaqat
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
			`DROP POLICY IF EXISTS tenant_isolation ON halaqat`,
			`ALTER TABLE halaqat NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE halaqat DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE halaqat`,
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

Build + commit `migrate: halaqat table + RLS`.

---

### Task 2: `students` migration + RLS

**Files:**
- Create: `api/internal/migrate/migrations/20260508000004_students.go`

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
			`CREATE TABLE students (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    organization_id UUID NOT NULL REFERENCES organizations(id),
			    halaqah_id UUID REFERENCES halaqat(id),
			    name TEXT NOT NULL,
			    age INT,
			    parent_phone TEXT,
			    parent_email TEXT,
			    hifz_level TEXT,
			    status TEXT NOT NULL DEFAULT 'active'
			      CHECK (status IN ('active', 'inactive')),
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_students_organization_id ON students(organization_id)`,
			`CREATE INDEX idx_students_halaqah_id ON students(halaqah_id)`,
			`GRANT SELECT, INSERT, UPDATE, DELETE ON students TO mutqin_app`,
			`ALTER TABLE students ENABLE ROW LEVEL SECURITY`,
			`ALTER TABLE students FORCE ROW LEVEL SECURITY`,
			`CREATE POLICY tenant_isolation ON students
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
			`DROP POLICY IF EXISTS tenant_isolation ON students`,
			`ALTER TABLE students NO FORCE ROW LEVEL SECURITY`,
			`ALTER TABLE students DISABLE ROW LEVEL SECURITY`,
			`DROP TABLE students`,
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

Build + commit `migrate: students table + RLS`.

---

### Task 3: `Halaqah` + `Student` Bun models

**Files:**
- Create: `api/internal/model/halaqah.go`
- Create: `api/internal/model/student.go`

```go
// api/internal/model/halaqah.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Halaqah struct {
	bun.BaseModel `bun:"table:halaqat,alias:h"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	Name           string     `bun:"name,notnull"`
	TeacherID      *uuid.UUID `bun:"teacher_id,type:uuid"`
	Schedule       []byte     `bun:"schedule,type:jsonb"`
	MaxCapacity    int        `bun:"max_capacity,notnull,nullzero,default:30"`
	Status         string     `bun:"status,notnull,nullzero,default:'active'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
```

```go
// api/internal/model/student.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Student struct {
	bun.BaseModel `bun:"table:students,alias:s"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	HalaqahID      *uuid.UUID `bun:"halaqah_id,type:uuid"`
	Name           string     `bun:"name,notnull"`
	Age            *int       `bun:"age"`
	ParentPhone    *string    `bun:"parent_phone"`
	ParentEmail    *string    `bun:"parent_email"`
	HifzLevel      *string    `bun:"hifz_level"`
	Status         string     `bun:"status,notnull,nullzero,default:'active'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
```

Build + commit `model: Halaqah + Student Bun models`.

---

### Task 4: `HalaqahRepo` (TDD)

**Files:**
- Create: `api/internal/repo/halaqah.go`
- Create: `api/internal/repo/halaqah_test.go`

Methods: `Create`, `GetByID(ctx, id)`, `ListByOrg(ctx, limit, offset)`, `Update(ctx, *Halaqah)`.

Tests use `testAdmin` for setup (org + teacher) and `tenant.With(ctx, orgID)` for the tenant-scoped queries on `testDB` (mutqin_app, RLS-subject).

```go
// api/internal/repo/halaqah_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestHalaqahRepo_CreateAndGet(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "H Org", Slug: "h-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewHalaqahRepo(testAdmin)
	h := &model.Halaqah{
		OrganizationID: org.ID,
		Name:           "Halaqah Al-Fajr",
		MaxCapacity:    25,
	}
	if err := r.Create(ctx, h); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.ID == uuid.Nil {
		t.Fatal("ID not populated")
	}

	got, err := r.GetByID(tenant.With(ctx, org.ID), h.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "Halaqah Al-Fajr" {
		t.Fatalf("name: %s", got.Name)
	}
}

func TestHalaqahRepo_ListByOrg_TenantScoped(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "h-a", Country: "SO", Tier: "free", Status: "active"}
	orgB := &model.Organization{Name: "B", Slug: "h-b", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB)

	r := repo.NewHalaqahRepo(testAdmin)
	_ = r.Create(ctx, &model.Halaqah{OrganizationID: orgA.ID, Name: "A1"})
	_ = r.Create(ctx, &model.Halaqah{OrganizationID: orgA.ID, Name: "A2"})
	_ = r.Create(ctx, &model.Halaqah{OrganizationID: orgB.ID, Name: "B1"})

	gotA, err := r.ListByOrg(tenant.With(ctx, orgA.ID), 100, 0)
	if err != nil {
		t.Fatalf("ListByOrg A: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("orgA list: want 2, got %d", len(gotA))
	}
	gotB, err := r.ListByOrg(tenant.With(ctx, orgB.ID), 100, 0)
	if err != nil {
		t.Fatalf("ListByOrg B: %v", err)
	}
	if len(gotB) != 1 {
		t.Fatalf("orgB list: want 1, got %d", len(gotB))
	}
}

func TestHalaqahRepo_Update_Status(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "U", Slug: "h-u", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, org)
	r := repo.NewHalaqahRepo(testAdmin)
	h := &model.Halaqah{OrganizationID: org.ID, Name: "X", MaxCapacity: 30}
	_ = r.Create(ctx, h)

	h.Status = "inactive"
	h.Name = "X (deactivated)"
	if err := r.Update(tenant.With(ctx, org.ID), h); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, err := r.GetByID(tenant.With(ctx, org.ID), h.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "inactive" {
		t.Fatalf("status=%s want inactive", got.Status)
	}
	if got.Name != "X (deactivated)" {
		t.Fatalf("name=%s", got.Name)
	}
}
```

Implementation:

```go
// api/internal/repo/halaqah.go
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

type HalaqahRepo struct {
	db bun.IDB
}

func NewHalaqahRepo(db bun.IDB) *HalaqahRepo {
	return &HalaqahRepo{db: db}
}

func (r *HalaqahRepo) Create(ctx context.Context, h *model.Halaqah) error {
	_, err := r.db.NewInsert().Model(h).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert halaqah: %w", err)
	}
	return nil
}

func (r *HalaqahRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error) {
	h := new(model.Halaqah)
	err := r.db.NewSelect().Model(h).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select halaqah: %w", err)
	}
	return h, nil
}

func (r *HalaqahRepo) ListByOrg(ctx context.Context, limit, offset int) ([]model.Halaqah, error) {
	var rows []model.Halaqah
	err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list halaqat: %w", err)
	}
	return rows, nil
}

func (r *HalaqahRepo) Update(ctx context.Context, h *model.Halaqah) error {
	res, err := r.db.NewUpdate().
		Model(h).
		Set("name = ?", h.Name).
		Set("teacher_id = ?", h.TeacherID).
		Set("max_capacity = ?", h.MaxCapacity).
		Set("status = ?", h.Status).
		Where("id = ?", h.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update halaqah: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

Run tests, commit `repo: HalaqahRepo (Create, GetByID, ListByOrg, Update)`.

---

### Task 5: `StudentRepo` (TDD)

**Files:**
- Create: `api/internal/repo/student.go`
- Create: `api/internal/repo/student_test.go`

Methods: `Create`, `GetByID`, `ListByHalaqah(ctx, halaqahID, limit, offset)`, `ListByOrg(ctx, limit, offset)`, `Update`, `Transfer(ctx, studentID, newHalaqahID uuid.UUID)`.

```go
// api/internal/repo/student_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestStudentRepo_CreateAndListByHalaqah(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "S Org", Slug: "s-org", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, org)
	h := &model.Halaqah{OrganizationID: org.ID, Name: "Hal A", MaxCapacity: 30}
	_ = repo.NewHalaqahRepo(testAdmin).Create(ctx, h)

	r := repo.NewStudentRepo(testAdmin)
	for _, n := range []string{"Ali", "Omar", "Bilal"} {
		s := &model.Student{OrganizationID: org.ID, HalaqahID: &h.ID, Name: n}
		if err := r.Create(ctx, s); err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}

	got, err := r.ListByHalaqah(tenant.With(ctx, org.ID), h.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListByHalaqah: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestStudentRepo_Transfer(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "T Org", Slug: "t-org", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, org)
	hA := &model.Halaqah{OrganizationID: org.ID, Name: "A", MaxCapacity: 30}
	hB := &model.Halaqah{OrganizationID: org.ID, Name: "B", MaxCapacity: 30}
	_ = repo.NewHalaqahRepo(testAdmin).Create(ctx, hA)
	_ = repo.NewHalaqahRepo(testAdmin).Create(ctx, hB)
	r := repo.NewStudentRepo(testAdmin)
	s := &model.Student{OrganizationID: org.ID, HalaqahID: &hA.ID, Name: "Khalid"}
	_ = r.Create(ctx, s)

	if err := r.Transfer(tenant.With(ctx, org.ID), s.ID, hB.ID); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	got, err := r.GetByID(tenant.With(ctx, org.ID), s.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.HalaqahID == nil || *got.HalaqahID != hB.ID {
		t.Fatalf("halaqah: %v want %s", got.HalaqahID, hB.ID)
	}
	// Lists narrow correctly.
	listA, _ := r.ListByHalaqah(tenant.With(ctx, org.ID), hA.ID, 100, 0)
	listB, _ := r.ListByHalaqah(tenant.With(ctx, org.ID), hB.ID, 100, 0)
	if len(listA) != 0 || len(listB) != 1 {
		t.Fatalf("after transfer: A=%d B=%d (want 0/1)", len(listA), len(listB))
	}
}
```

Implementation:

```go
// api/internal/repo/student.go
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

type StudentRepo struct {
	db bun.IDB
}

func NewStudentRepo(db bun.IDB) *StudentRepo {
	return &StudentRepo{db: db}
}

func (r *StudentRepo) Create(ctx context.Context, s *model.Student) error {
	_, err := r.db.NewInsert().Model(s).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert student: %w", err)
	}
	return nil
}

func (r *StudentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Student, error) {
	s := new(model.Student)
	err := r.db.NewSelect().Model(s).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select student: %w", err)
	}
	return s, nil
}

func (r *StudentRepo) ListByHalaqah(ctx context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error) {
	var rows []model.Student
	err := r.db.NewSelect().
		Model(&rows).
		Where("halaqah_id = ?", halaqahID).
		OrderExpr("name ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list students by halaqah: %w", err)
	}
	return rows, nil
}

func (r *StudentRepo) ListByOrg(ctx context.Context, limit, offset int) ([]model.Student, error) {
	var rows []model.Student
	err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("name ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list students: %w", err)
	}
	return rows, nil
}

func (r *StudentRepo) Update(ctx context.Context, s *model.Student) error {
	res, err := r.db.NewUpdate().
		Model(s).
		Set("name = ?", s.Name).
		Set("age = ?", s.Age).
		Set("parent_phone = ?", s.ParentPhone).
		Set("parent_email = ?", s.ParentEmail).
		Set("hifz_level = ?", s.HifzLevel).
		Set("status = ?", s.Status).
		Where("id = ?", s.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update student: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *StudentRepo) Transfer(ctx context.Context, studentID, newHalaqahID uuid.UUID) error {
	res, err := r.db.NewUpdate().
		Model((*model.Student)(nil)).
		Set("halaqah_id = ?", newHalaqahID).
		Where("id = ?", studentID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("transfer student: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

Run tests, commit `repo: StudentRepo (CRUD + Transfer)`.

---

### Task 6: `AuthAsTenant` middleware

**Files:**
- Create: `api/internal/middleware/auth_as_tenant.go`
- Create: `api/internal/middleware/auth_as_tenant_test.go`

Bridges `auth.Identity.OrgID` → `tenant.With` so RLS context fires for authenticated requests with no Host-based slug.

```go
// api/internal/middleware/auth_as_tenant_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestAuthAsTenant_PromotesIdentityToTenant(t *testing.T) {
	mw := middleware.AuthAsTenant
	orgID := uuid.New()
	var got uuid.UUID
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !ok || got != orgID {
		t.Fatalf("got=(%v,%v) want (%s,true)", got, ok, orgID)
	}
}

func TestAuthAsTenant_DoesNotOverrideExistingTenant(t *testing.T) {
	hostOrg := uuid.New()
	authOrg := uuid.New()
	mw := middleware.AuthAsTenant

	var got uuid.UUID
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := tenant.With(req.Context(), hostOrg)
	ctx = auth.With(ctx, auth.Identity{UserID: uuid.New(), OrgID: &authOrg, Role: "center_admin"})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got != hostOrg {
		t.Fatalf("got %s want hostOrg %s (existing tenant should not be overridden)", got, hostOrg)
	}
}

func TestAuthAsTenant_PassesThroughForSuperAdmin(t *testing.T) {
	mw := middleware.AuthAsTenant
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = tenant.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: nil, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ok {
		t.Fatal("super_admin should not have a tenant in ctx")
	}
}
```

Implementation:

```go
// api/internal/middleware/auth_as_tenant.go
package middleware

import (
	"net/http"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

// AuthAsTenant promotes an authenticated user's organization id from the
// auth.Identity into tenant.With, so the existing RLSContext middleware
// downstream sees the tenant and runs SET LOCAL accordingly.
//
// Order of precedence: an explicit tenant set earlier (e.g., by the Tenant
// resolver from a host header) wins; this middleware only fills in when no
// tenant has been set yet AND the JWT belongs to a tenant-scoped role.
//
// super_admin requests intentionally pass through without a tenant — they
// operate cross-tenant on the admin handle, not via RLS.
func AuthAsTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if _, has := tenant.From(ctx); has {
			next.ServeHTTP(w, r)
			return
		}
		id, ok := auth.From(ctx)
		if !ok || id.OrgID == nil {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(tenant.With(ctx, *id.OrgID)))
	})
}
```

Run tests, commit `middleware: AuthAsTenant (bridges JWT org_id into tenant ctx)`.

---

### Task 7: Extend `InviteService` with `GenerateForTeacher`

**Files:**
- Modify: `api/internal/service/invite.go`
- Modify: `api/internal/service/invite_test.go`

```go
func (s *InviteService) GenerateForTeacher(ctx context.Context, actorID, orgID uuid.UUID) (*model.Invite, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	inv := &model.Invite{
		OrganizationID: orgID,
		Role:           "teacher",
		Token:          hex.EncodeToString(tokenBytes),
		ExpiresAt:      time.Now().Add(inviteTTL),
		CreatedBy:      actorID,
	}
	if err := s.invites.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create teacher invite: %w", err)
	}

	tt := "invite"
	tid := inv.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "generate_teacher_invite",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return inv, nil
}
```

Add a test mirroring `TestInviteService_Generate_HappyPath` for the teacher flow. Commit `service: InviteService.GenerateForTeacher`.

---

### Task 8: `HalaqahService` (TDD with mocks)

**Files:**
- Create: `api/internal/service/halaqah.go`
- Create: `api/internal/service/halaqah_test.go`

Methods: `Create`, `GetByID`, `List` (limit/offset), `Update`, `Deactivate(ctx, actor, halaqahID)`. Each mutating method audits.

Plan note: services accept tenant ctx (caller-supplied). They never set tenant themselves — that's the middleware's job.

Sample test:

```go
func TestHalaqahService_Create_RecordsAudit(t *testing.T) {
	halaqat := newStubHalaqahRepo()
	audit := &stubAuditRepo{}
	svc := service.NewHalaqahService(halaqat, audit)

	actor := uuid.New()
	orgID := uuid.New()
	h, err := svc.Create(context.Background(), actor, orgID, service.CreateHalaqahInput{
		Name: "Halaqah Al-Asr", MaxCapacity: 25,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if h.OrganizationID != orgID {
		t.Fatal("org id mismatch")
	}
	if len(audit.logs) != 1 || audit.logs[0].Action != "create_halaqah" {
		t.Fatalf("audit: %+v", audit.logs)
	}
}
```

Implement with the standard service pattern: package-private repo interface, audit on every mutation.

Commit `service: HalaqahService (Create/Get/List/Update/Deactivate + audit)`.

---

### Task 9: `StudentService` (TDD with mocks)

**Files:**
- Create: `api/internal/service/student.go`
- Create: `api/internal/service/student_test.go`

Methods: `Enroll(ctx, actor, orgID, halaqahID, in)`, `ListByHalaqah(ctx, halaqahID, limit, offset)`, `Transfer(ctx, actor, studentID, newHalaqahID)`, `Deactivate`.

`Enroll` validates the target halaqah exists and is in the same org (via repo lookup). `Transfer` validates new halaqah is in same org.

Tests with mocks for student repo + halaqah repo + audit.

Commit `service: StudentService (Enroll, List, Transfer, Deactivate + audit)`.

---

### Task 10: HTTP handlers — `halaqat`, `students`, `teachers`

**Files:**
- Create: `api/internal/handler/api/halaqat.go` + test
- Create: `api/internal/handler/api/students.go` + test
- Create: `api/internal/handler/api/teachers.go` + test

Routes (mounted later in Task 11):
- `POST /api/v1/halaqat` — center_admin creates halaqah
- `GET /api/v1/halaqat` — center_admin lists tenant's halaqat
- `GET /api/v1/halaqat/{id}` — center_admin gets one
- `PATCH /api/v1/halaqat/{id}` — center_admin updates / assigns teacher
- `POST /api/v1/halaqat/{id}/students` — center_admin enrolls
- `GET /api/v1/halaqat/{id}/students` — center_admin OR teacher lists (RLS narrows; teacher can only access their assigned halaqah by halaqah lookup, but the tenant filter handles that for the ListByHalaqah call)
- `POST /api/v1/students/{id}/transfer` — center_admin transfers
- `POST /api/v1/teachers/invite` — center_admin generates teacher invite

Each handler: parses body, pulls actor + orgID from `auth.From(ctx)`, calls service.

Commit per handler file: `handler: halaqat`, `handler: students`, `handler: teachers (invite gen)`.

---

### Task 11: Wire `cmd/server/main.go`

**Files:**
- Modify: `api/cmd/server/main.go`

After Plan I's wiring, add:

```go
	halaqahSvc := service.NewHalaqahService(repo.NewHalaqahRepo(appDB), auditRepo)
	studentSvc := service.NewStudentService(repo.NewStudentRepo(appDB), repo.NewHalaqahRepo(appDB), auditRepo)
	halaqatH := apihandler.NewHalaqatHandler(halaqahSvc)
	studentsH := apihandler.NewStudentsHandler(studentSvc)
	teachersH := apihandler.NewTeachersHandler(inviteSvc)
```

Add `AuthAsTenant` to the chain (after `Auth`, before `RLSContext`):

```go
	r.Use(middleware.Auth(jwtVerifier))
	r.Use(middleware.AuthAsTenant)
	r.Use(middleware.RLSContext(middleware.NewBunRunner(appDB)))
```

Add the center-admin route group:

```go
	r.Group(func(ca chi.Router) {
		ca.Use(middleware.Role("center_admin", "super_admin"))
		ca.Post("/api/v1/halaqat", halaqatH.Create)
		ca.Get("/api/v1/halaqat", halaqatH.List)
		ca.Get("/api/v1/halaqat/{id}", halaqatH.Get)
		ca.Patch("/api/v1/halaqat/{id}", halaqatH.Update)
		ca.Post("/api/v1/halaqat/{id}/students", studentsH.Enroll)
		ca.Post("/api/v1/students/{id}/transfer", studentsH.Transfer)
		ca.Post("/api/v1/teachers/invite", teachersH.GenerateInvite)
	})
```

Add a teacher route group for read-only halaqah/student listing:

```go
	r.Group(func(tr chi.Router) {
		tr.Use(middleware.Role("teacher", "center_admin", "super_admin"))
		tr.Get("/api/v1/halaqat/{id}/students", studentsH.ListByHalaqah)
	})
```

Commit `cmd: wire HalaqahService/StudentService/teacher invite + center_admin/teacher route groups + AuthAsTenant`.

---

### Task 12: End-to-end smoke

Bring up stack. Bootstrap super_admin. Login → create org. Generate center_admin invite. Accept → verify → JWT. As center_admin: create halaqah, generate teacher invite, accept → verify → JWT. As center_admin: enroll students, transfer one. As teacher: list students of own halaqah.

Verify in DB: halaqat row count, students row count, halaqah_id correctly set after transfer.

Tear down. Commit nothing (verification only).

---

### Task 13: Final verify + push

Tests + vet + push to origin + gitlab. Open PR + MR with summary.

---

## Self-Review

**Coverage:**
- FR14 (halaqah create) ✓ Tasks 4, 8, 10
- FR15 (assign teacher) ✓ Update endpoint includes teacher_id
- FR16 (enroll students) ✓ Tasks 5, 9, 10
- FR17 (transfer) ✓ Repo + service + handler
- FR18 (list halaqat) ✓
- FR19 (deactivate halaqah) ✓ Update with status='inactive'
- FR20 (teacher invite) ✓ Task 7 + 10
- FR21 (teacher activates) ✓ Existing accept-invite flow handles `role=teacher`
- FR23 (RBAC) ✓ Task 11 route groups
- FR24 (deactivate teacher) — `UserRepo.Update` allows status change; HTTP endpoint deferred (small follow-up; mechanically same as halaqah deactivate)

**Placeholder scan:** None. Each task has the actual code or refers to a clear pattern from earlier tasks.

**Type / signature consistency:**
- `HalaqahRepo.Update(ctx, *Halaqah)`, `Update(ctx, *Student)`, `UserRepo.Update` all use the same shape.
- `service.NewHalaqahService(halaqat, audit)`, `NewStudentService(students, halaqat, audit)` — the latter takes halaqahRepo for cross-validation in Enroll/Transfer.
- Handlers consume services via interfaces (per the established pattern from Plan I); test stubs swap in.

**Scope:** 13 tasks. Substantive: 1, 2, 4, 5, 6, 8, 9, 10, 11. Trivial: 3 (models). Verification: 12, 13.
