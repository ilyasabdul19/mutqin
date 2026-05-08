# Plan I — Platform Administration & Invite Flow

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Super-admin org CRUD, invite generation for center admins, invite-accept flow that integrates with Plan H's OTP login, and an audit log of super-admin actions.

**Architecture:** A new `OrganizationService` and `InviteService` sit in `internal/service/`. Two new repos: `InviteRepo` (Create, GetByToken, MarkUsed) and `AuditLogRepo` (Create, ListByActor). The platform handler at `/api/v1/platform/*` and `/api/v1/organizations/*` routes are gated by `Auth` + `Role("super_admin")`. The invite-accept endpoint at `/api/v1/auth/invite/accept` is public — it consumes a token, pre-creates the user with `status=pending`, marks the invite used, and sends the OTP. The user then verifies via the existing `/auth/otp/verify`, which transitions `pending → active`.

**Audit log** is global (not tenant-scoped) — super-admin actions cross tenants by definition. Services explicitly call `AuditLogRepo.Create` after successful mutations (no magic middleware).

**Tech Stack:**
- Existing: Bun, Chi, slog, JWT, OTP, RLS pattern
- No new external deps

**Out of scope (deferred):**
- Suspend / tier change endpoints (a small follow-up — same shape as Update)
- Platform-wide dashboard with aggregate stats (Plan M — query-heavy, deserves own plan)
- Cloudflare onboarding integration on org-create (operator runs `cmd/onboard --slug=...` manually for now; integration lands later when org-create UX is finalized)
- Email-based invite delivery (invite tokens go out-of-band for now; email send wires in when SMTP lands)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/migrate/migrations/20260508000002_audit_log.go` | `audit_log` table — global, not RLS-subject |
| `api/internal/model/audit_log.go` | Bun model |
| `api/internal/repo/invite.go` | `InviteRepo` (Create, GetByToken, MarkUsed) |
| `api/internal/repo/invite_test.go` | Integration tests |
| `api/internal/repo/audit_log.go` | `AuditLogRepo` (Create, ListByActor) |
| `api/internal/repo/audit_log_test.go` | Integration tests |
| `api/internal/service/organization.go` | Create / Get / List orgs |
| `api/internal/service/organization_test.go` | Tests with mocks |
| `api/internal/service/invite.go` | GenerateForCenterAdmin / AcceptInvite |
| `api/internal/service/invite_test.go` | Tests with mocks |
| `api/internal/handler/api/platform.go` | HTTP: orgs CRUD + generate-invite |
| `api/internal/handler/api/platform_test.go` | Handler tests with mocked services |
| `api/internal/handler/api/invite.go` | HTTP: POST /auth/invite/accept |
| `api/internal/handler/api/invite_test.go` | Handler tests |

### Modified

| Path | Change |
|------|--------|
| `api/internal/service/auth.go` | `VerifyOTP` activates `pending` users on success (atomic UPDATE) |
| `api/internal/service/auth_test.go` | New test for pending → active transition |
| `api/internal/repo/user.go` | Add `Update` (UPDATE row by id, returning *) |
| `api/cmd/server/main.go` | Wire `OrganizationService`, `InviteService`, `AuditLogRepo`; mount platform routes with `Auth` + `Role("super_admin")` guard; mount public invite-accept |

### Deleted
None.

---

## Tasks

### Task 1: `audit_log` migration

**Files:**
- Create: `api/internal/migrate/migrations/20260508000002_audit_log.go`

- [ ] **Step 1: Write the migration**

```go
// api/internal/migrate/migrations/20260508000002_audit_log.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		queries := []string{
			`CREATE TABLE audit_log (
			    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			    actor_id UUID REFERENCES users(id),
			    action TEXT NOT NULL,
			    target_type TEXT,
			    target_id UUID,
			    details JSONB,
			    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
			)`,
			`CREATE INDEX idx_audit_log_actor_id ON audit_log(actor_id)`,
			`CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC)`,
			// audit_log is global — super_admin actions cross tenants by definition.
			// Grant read+write to mutqin_app; do NOT enable RLS.
			`GRANT SELECT, INSERT ON audit_log TO mutqin_app`,
		}
		for _, q := range queries {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, `DROP TABLE audit_log`)
		return err
	})
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/migrate/migrations/20260508000002_audit_log.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "migrate: audit_log (global, not RLS-subject)"
```

---

### Task 2: `AuditLog` Bun model

**Files:**
- Create: `api/internal/model/audit_log.go`

- [ ] **Step 1: Write the model**

```go
// api/internal/model/audit_log.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AuditLog struct {
	bun.BaseModel `bun:"table:audit_log,alias:al"`

	ID         uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	ActorID    *uuid.UUID `bun:"actor_id,type:uuid"`
	Action     string     `bun:"action,notnull"`
	TargetType *string    `bun:"target_type"`
	TargetID   *uuid.UUID `bun:"target_id,type:uuid"`
	Details    []byte     `bun:"details,type:jsonb"`
	CreatedAt  time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
```

- [ ] **Step 2: Build + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go build ./internal/model
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/model/audit_log.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "model: AuditLog Bun model"
```

---

### Task 3: `AuditLogRepo` (TDD)

**Files:**
- Create: `api/internal/repo/audit_log.go`
- Create: `api/internal/repo/audit_log_test.go`

- [ ] **Step 1: Failing test**

```go
// api/internal/repo/audit_log_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestAuditLogRepo_CreateAndList(t *testing.T) {
	t.Cleanup(func() { truncateAll(t); _, _ = testAdmin.ExecContext(context.Background(), "TRUNCATE audit_log") })
	ctx := context.Background()

	// Need an actor: create org + user via admin handle.
	org := &model.Organization{Name: "AOrg", Slug: "audit-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	actor := &model.User{Name: "Actor", Email: ptrString("audit-actor@x"), Role: "super_admin", Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, actor); err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := repo.NewAuditLogRepo(testAdmin)

	tt := "organization"
	tid := org.ID
	row := &model.AuditLog{
		ActorID:    &actor.ID,
		Action:     "create_organization",
		TargetType: &tt,
		TargetID:   &tid,
		Details:    []byte(`{"slug":"audit-org"}`),
	}
	if err := r.Create(ctx, row); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if row.ID == uuid.Nil {
		t.Fatal("ID not populated")
	}

	got, err := r.ListByActor(ctx, actor.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListByActor: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Action != "create_organization" {
		t.Fatalf("action=%s", got[0].Action)
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/repo/audit_log.go
package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type AuditLogRepo struct {
	db bun.IDB
}

func NewAuditLogRepo(db bun.IDB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, row *model.AuditLog) error {
	_, err := r.db.NewInsert().Model(row).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

func (r *AuditLogRepo) ListByActor(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	var rows []model.AuditLog
	err := r.db.NewSelect().
		Model(&rows).
		Where("actor_id = ?", actorID).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list audit_log: %w", err)
	}
	return rows, nil
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test -run TestAuditLog ./internal/repo/...
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/repo/audit_log.go api/internal/repo/audit_log_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "repo: AuditLogRepo (Create, ListByActor)"
```

---

### Task 4: `InviteRepo` (TDD)

**Files:**
- Create: `api/internal/repo/invite.go`
- Create: `api/internal/repo/invite_test.go`

- [ ] **Step 1: Failing test**

```go
// api/internal/repo/invite_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestInviteRepo_CreateAndGetByToken(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "InvOrg", Slug: "inv-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	creator := &model.User{Name: "Creator", Email: ptrString("inv-creator@x"), Role: "super_admin", Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, creator); err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := repo.NewInviteRepo(testAdmin)
	inv := &model.Invite{
		OrganizationID: org.ID,
		Role:           "center_admin",
		Token:          "tok-abc",
		ExpiresAt:      time.Now().Add(72 * time.Hour),
		CreatedBy:      creator.ID,
	}
	if err := r.Create(ctx, inv); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.GetByToken(ctx, "tok-abc")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if got.OrganizationID != org.ID {
		t.Fatalf("org_id mismatch")
	}
	if got.Role != "center_admin" {
		t.Fatalf("role=%s", got.Role)
	}
}

func TestInviteRepo_GetByToken_NotFound(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewInviteRepo(testAdmin)
	if _, err := r.GetByToken(ctx, "ghost-token"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestInviteRepo_MarkUsed(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "MOrg", Slug: "m-org", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, org)
	creator := &model.User{Name: "C", Email: ptrString("m-c@x"), Role: "super_admin", Status: "active", Language: "ar"}
	_ = repo.NewUserRepo(testAdmin).Create(ctx, creator)

	r := repo.NewInviteRepo(testAdmin)
	inv := &model.Invite{
		OrganizationID: org.ID,
		Role:           "center_admin",
		Token:          "tok-mark",
		ExpiresAt:      time.Now().Add(72 * time.Hour),
		CreatedBy:      creator.ID,
	}
	_ = r.Create(ctx, inv)

	// Need tenant ctx because invites is RLS-subject.
	tctx := tenant.With(ctx, org.ID)
	if err := r.MarkUsed(tctx, inv.ID); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}

	got, err := r.GetByToken(ctx, "tok-mark")
	if err != nil {
		t.Fatalf("GetByToken: %v", err)
	}
	if got.UsedAt == nil {
		t.Fatal("UsedAt not set")
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/repo/invite.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type InviteRepo struct {
	db bun.IDB
}

func NewInviteRepo(db bun.IDB) *InviteRepo {
	return &InviteRepo{db: db}
}

func (r *InviteRepo) Create(ctx context.Context, inv *model.Invite) error {
	_, err := r.db.NewInsert().Model(inv).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert invite: %w", err)
	}
	return nil
}

// GetByToken looks up an invite by token without a tenant filter — invites are
// looked up before the tenant ctx is known. Caller passes the admin handle.
func (r *InviteRepo) GetByToken(ctx context.Context, token string) (*model.Invite, error) {
	inv := new(model.Invite)
	err := r.db.NewSelect().Model(inv).Where("token = ?", token).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select invite: %w", err)
	}
	return inv, nil
}

func (r *InviteRepo) MarkUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*model.Invite)(nil)).
		Set("used_at = ?", now).
		Where("id = ?", id).
		Where("used_at IS NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test -run TestInviteRepo ./internal/repo/...
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/repo/invite.go api/internal/repo/invite_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "repo: InviteRepo (Create, GetByToken, MarkUsed)"
```

---

### Task 5: `UserRepo.Update` (extends Plan H repo)

**Files:**
- Modify: `api/internal/repo/user.go`
- Modify: `api/internal/repo/user_test.go`

- [ ] **Step 1: Append the method to user.go**

After `GetByEmailGlobal`:

```go
// Update updates a user record by ID. The status, name, role, and language
// fields are mutable. Returns ErrNotFound if no row matched.
func (r *UserRepo) Update(ctx context.Context, u *model.User) error {
	res, err := r.db.NewUpdate().
		Model(u).
		Set("name = ?", u.Name).
		Set("role = ?", u.Role).
		Set("status = ?", u.Status).
		Set("language = ?", u.Language).
		Where("id = ?", u.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 2: Append a test**

```go
func TestUserRepo_Update_Status(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "U", Slug: "upd", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}
	users := repo.NewUserRepo(testAdmin)
	u := &model.User{
		Name: "P", Email: ptrString("upd-p@x"), Role: "teacher",
		OrganizationID: ptrUUID(org.ID), Status: "pending", Language: "ar",
	}
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}

	u.Status = "active"
	if err := users.Update(ctx, u); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := users.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("getbyid: %v", err)
	}
	if got.Status != "active" {
		t.Fatalf("status=%s want active", got.Status)
	}
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test -run TestUserRepo ./internal/repo/... 2>&1 | tail -5
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/repo/user.go api/internal/repo/user_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "repo: UserRepo.Update (status, name, role, language)"
```

---

### Task 6: `OrganizationService` (TDD with mocks)

**Files:**
- Create: `api/internal/service/organization.go`
- Create: `api/internal/service/organization_test.go`

The service is thin over the repo. It records audit log entries on Create.

- [ ] **Step 1: Failing tests**

```go
// api/internal/service/organization_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubOrgRepo struct {
	created *model.Organization
	stored  map[string]*model.Organization
	listed  []model.Organization
}

func newStubOrgRepo() *stubOrgRepo {
	return &stubOrgRepo{stored: map[string]*model.Organization{}}
}

func (s *stubOrgRepo) Create(_ context.Context, o *model.Organization) error {
	o.ID = uuid.New()
	s.created = o
	s.stored[o.Slug] = o
	return nil
}
func (s *stubOrgRepo) GetBySlug(_ context.Context, slug string) (*model.Organization, error) {
	if o, ok := s.stored[slug]; ok {
		return o, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubOrgRepo) List(_ context.Context, limit, offset int) ([]model.Organization, error) {
	return s.listed, nil
}

type stubAuditRepo struct{ logs []model.AuditLog }

func (s *stubAuditRepo) Create(_ context.Context, row *model.AuditLog) error {
	row.ID = uuid.New()
	s.logs = append(s.logs, *row)
	return nil
}

func TestOrgService_Create_RecordsAudit(t *testing.T) {
	orgs := newStubOrgRepo()
	audit := &stubAuditRepo{}
	svc := service.NewOrganizationService(orgs, audit)

	actor := uuid.New()
	o, err := svc.Create(context.Background(), actor, service.CreateOrgInput{
		Name: "Markaz X", Slug: "markaz-x", Country: "SO",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if orgs.created != o {
		t.Fatal("repo Create not called with returned org")
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
	if audit.logs[0].Action != "create_organization" {
		t.Fatalf("action=%s", audit.logs[0].Action)
	}
	if audit.logs[0].ActorID == nil || *audit.logs[0].ActorID != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestOrgService_Create_DefaultsTierAndStatus(t *testing.T) {
	orgs := newStubOrgRepo()
	svc := service.NewOrganizationService(orgs, &stubAuditRepo{})
	o, err := svc.Create(context.Background(), uuid.New(), service.CreateOrgInput{
		Name: "X", Slug: "x", Country: "SO",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if o.Tier != "free" {
		t.Fatalf("tier=%s want free", o.Tier)
	}
	if o.Status != "active" {
		t.Fatalf("status=%s want active", o.Status)
	}
}

func TestOrgService_GetBySlug(t *testing.T) {
	orgs := newStubOrgRepo()
	orgs.stored["foo"] = &model.Organization{ID: uuid.New(), Slug: "foo"}
	svc := service.NewOrganizationService(orgs, &stubAuditRepo{})
	got, err := svc.GetBySlug(context.Background(), "foo")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.Slug != "foo" {
		t.Fatalf("slug=%s", got.Slug)
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/service/organization.go
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

type orgRepo interface {
	Create(ctx context.Context, o *model.Organization) error
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	List(ctx context.Context, limit, offset int) ([]model.Organization, error)
}

type auditRepo interface {
	Create(ctx context.Context, row *model.AuditLog) error
}

type OrganizationService struct {
	orgs  orgRepo
	audit auditRepo
}

func NewOrganizationService(orgs orgRepo, audit auditRepo) *OrganizationService {
	return &OrganizationService{orgs: orgs, audit: audit}
}

// CreateOrgInput is the validated payload for creating a new organization.
type CreateOrgInput struct {
	Name        string
	Slug        string
	Country     string
	City        *string
	Description *string
	Tier        string // empty → "free"
}

func (s *OrganizationService) Create(ctx context.Context, actorID uuid.UUID, in CreateOrgInput) (*model.Organization, error) {
	o := &model.Organization{
		Name:        in.Name,
		Slug:        in.Slug,
		Country:     in.Country,
		City:        in.City,
		Description: in.Description,
		Tier:        in.Tier,
		Status:      "active",
	}
	if o.Tier == "" {
		o.Tier = "free"
	}
	if err := s.orgs.Create(ctx, o); err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	details, _ := json.Marshal(map[string]string{"slug": o.Slug, "name": o.Name})
	tt := "organization"
	tid := o.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "create_organization",
		TargetType: &tt,
		TargetID:   &tid,
		Details:    details,
	}) // audit failure does not roll back the org create
	return o, nil
}

func (s *OrganizationService) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	return s.orgs.GetBySlug(ctx, slug)
}

func (s *OrganizationService) List(ctx context.Context, limit, offset int) ([]model.Organization, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.orgs.List(ctx, limit, offset)
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test ./internal/service/... 2>&1 | tail -10
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/service/organization.go api/internal/service/organization_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "service: OrganizationService (Create + audit, GetBySlug, List)"
```

---

### Task 7: `InviteService` + `AuthService.VerifyOTP` activates pending users

**Files:**
- Create: `api/internal/service/invite.go`
- Create: `api/internal/service/invite_test.go`
- Modify: `api/internal/service/auth.go` — VerifyOTP transitions pending → active
- Modify: `api/internal/service/auth_test.go` — add test for pending → active

- [ ] **Step 1: Update AuthService.VerifyOTP**

In `api/internal/service/auth.go`, in `VerifyOTP`, replace the user lookup + sign block with:

```go
	u, err := s.users.GetByEmailGlobal(ctx, emailAddr)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}

	// On first verify of a pending user (created via invite), activate them.
	if u.Status == "pending" {
		u.Status = "active"
		if err := s.users.Update(ctx, u); err != nil {
			return "", fmt.Errorf("activate user: %w", err)
		}
	}

	tok, err := s.jwt.Sign(auth.Claims{
		UserID: u.ID,
		OrgID:  u.OrgID(),
		Role:   u.Role,
		TTL:    s.ttl,
	})
```

The `userRepo` interface in `auth.go` needs an `Update` method too:

```go
type userRepo interface {
	GetByEmailGlobal(ctx context.Context, email string) (*model.User, error)
	Update(ctx context.Context, u *model.User) error
}
```

In `auth_test.go`'s `stubUserRepo`, add an Update method:

```go
func (s *stubUserRepo) Update(_ context.Context, u *model.User) error {
	if existing, ok := s.users[*u.Email]; ok && existing.ID == u.ID {
		s.users[*u.Email] = u
		return nil
	}
	return repo.ErrNotFound
}
```

Add a test:

```go
func TestAuthService_VerifyOTP_ActivatesPendingUser(t *testing.T) {
	uid := uuid.New()
	orgID := uuid.New()
	users := &stubUserRepo{users: map[string]*model.User{
		"u@x": {ID: uid, Email: ptrStr("u@x"), Role: "center_admin", OrganizationID: ptrUUID(orgID), Status: "pending"},
	}}
	plain, hash, _ := auth.GenerateOTP()
	otps := &stubOtpRepo{consumed: &model.OtpCode{Email: "u@x", Code: hash}}
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	svc := service.NewAuthService(otps, users, email.NewLogSender(), iss, time.Hour)

	if _, err := svc.VerifyOTP(context.Background(), "u@x", plain); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if users.users["u@x"].Status != "active" {
		t.Fatalf("status=%s want active", users.users["u@x"].Status)
	}
}
```

- [ ] **Step 2: InviteService — failing test**

```go
// api/internal/service/invite_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

// --- stubs ---

type stubInviteRepo struct {
	byToken map[string]*model.Invite
	created *model.Invite
	usedID  uuid.UUID
}

func (s *stubInviteRepo) Create(_ context.Context, i *model.Invite) error {
	i.ID = uuid.New()
	if s.byToken == nil {
		s.byToken = map[string]*model.Invite{}
	}
	s.byToken[i.Token] = i
	s.created = i
	return nil
}
func (s *stubInviteRepo) GetByToken(_ context.Context, t string) (*model.Invite, error) {
	if i, ok := s.byToken[t]; ok {
		return i, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubInviteRepo) MarkUsed(_ context.Context, id uuid.UUID) error {
	s.usedID = id
	if s.byToken != nil {
		for _, inv := range s.byToken {
			if inv.ID == id {
				now := time.Now()
				inv.UsedAt = &now
			}
		}
	}
	return nil
}

type stubUserRepoFull struct {
	byEmail map[string]*model.User
	created *model.User
}

func (s *stubUserRepoFull) GetByEmailGlobal(_ context.Context, e string) (*model.User, error) {
	if u, ok := s.byEmail[e]; ok {
		return u, nil
	}
	return nil, repo.ErrNotFound
}
func (s *stubUserRepoFull) Create(_ context.Context, u *model.User) error {
	u.ID = uuid.New()
	if s.byEmail == nil {
		s.byEmail = map[string]*model.User{}
	}
	if u.Email != nil {
		s.byEmail[*u.Email] = u
	}
	s.created = u
	return nil
}
func (s *stubUserRepoFull) Update(_ context.Context, _ *model.User) error { return nil }

// --- tests ---

func TestInviteService_Generate_HappyPath(t *testing.T) {
	invs := &stubInviteRepo{}
	users := &stubUserRepoFull{}
	otps := &stubOtpRepo{}
	es := email.NewLogSender()
	audit := &stubAuditRepo{}

	svc := service.NewInviteService(invs, users, otps, es, audit)

	orgID := uuid.New()
	creator := uuid.New()
	inv, err := svc.GenerateForCenterAdmin(context.Background(), creator, orgID)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if inv.Token == "" {
		t.Fatal("empty token")
	}
	if invs.created != inv {
		t.Fatal("repo not called")
	}
	if inv.Role != "center_admin" {
		t.Fatalf("role=%s", inv.Role)
	}
	if inv.OrganizationID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if len(audit.logs) != 1 {
		t.Fatalf("audit logs=%d want 1", len(audit.logs))
	}
}

func TestInviteService_Accept_HappyPath(t *testing.T) {
	orgID := uuid.New()
	creator := uuid.New()
	now := time.Now()
	inv := &model.Invite{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Role:           "center_admin",
		Token:          "tok-1",
		ExpiresAt:      now.Add(time.Hour),
		CreatedBy:      creator,
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-1": inv}}
	users := &stubUserRepoFull{byEmail: map[string]*model.User{}}
	otps := &stubOtpRepo{}
	es := email.NewLogSender()
	audit := &stubAuditRepo{}

	svc := service.NewInviteService(invs, users, otps, es, audit)

	if err := svc.AcceptInvite(context.Background(), "tok-1", "newadmin@x.com", "Center Admin"); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// User pre-created with status=pending and role from invite.
	if users.created == nil {
		t.Fatal("user not created")
	}
	if users.created.Status != "pending" {
		t.Fatalf("status=%s want pending", users.created.Status)
	}
	if users.created.Role != "center_admin" {
		t.Fatalf("role=%s", users.created.Role)
	}
	if users.created.OrganizationID == nil || *users.created.OrganizationID != orgID {
		t.Fatal("org_id mismatch")
	}
	// OTP stored + email sent.
	if otps.created == nil {
		t.Fatal("OTP not stored")
	}
	if es.Last().To != "newadmin@x.com" {
		t.Fatalf("email To: %s", es.Last().To)
	}
	// Invite marked used.
	if inv.UsedAt == nil {
		t.Fatal("invite not marked used")
	}
}

func TestInviteService_Accept_BadToken(t *testing.T) {
	svc := service.NewInviteService(&stubInviteRepo{}, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	err := svc.AcceptInvite(context.Background(), "ghost", "x@y", "X")
	if !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_ExpiredInvite(t *testing.T) {
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-x", ExpiresAt: time.Now().Add(-time.Hour), CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-x": inv}}
	svc := service.NewInviteService(invs, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})

	if err := svc.AcceptInvite(context.Background(), "tok-x", "x@y", "X"); !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_AlreadyUsed(t *testing.T) {
	used := time.Now().Add(-time.Hour)
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-u", ExpiresAt: time.Now().Add(time.Hour), UsedAt: &used, CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-u": inv}}
	svc := service.NewInviteService(invs, &stubUserRepoFull{}, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	if err := svc.AcceptInvite(context.Background(), "tok-u", "x@y", "X"); !errors.Is(err, service.ErrInvalidInvite) {
		t.Fatalf("want ErrInvalidInvite, got %v", err)
	}
}

func TestInviteService_Accept_EmailAlreadyTaken(t *testing.T) {
	inv := &model.Invite{
		ID: uuid.New(), OrganizationID: uuid.New(), Role: "center_admin",
		Token: "tok-e", ExpiresAt: time.Now().Add(time.Hour), CreatedBy: uuid.New(),
	}
	invs := &stubInviteRepo{byToken: map[string]*model.Invite{"tok-e": inv}}
	users := &stubUserRepoFull{byEmail: map[string]*model.User{
		"taken@x": {ID: uuid.New(), Email: ptrStrSvc("taken@x")},
	}}
	svc := service.NewInviteService(invs, users, &stubOtpRepo{}, email.NewLogSender(), &stubAuditRepo{})
	if err := svc.AcceptInvite(context.Background(), "tok-e", "taken@x", "X"); !errors.Is(err, service.ErrEmailTaken) {
		t.Fatalf("want ErrEmailTaken, got %v", err)
	}
}

func ptrStrSvc(s string) *string { return &s }

// auth import is used only to pull GenerateOTP for symmetry with auth_test
var _ = auth.GenerateOTP
```

- [ ] **Step 3: Implement InviteService**

```go
// api/internal/service/invite.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

const inviteTTL = 72 * time.Hour

var (
	ErrInvalidInvite = errors.New("service: invalid invite")
	ErrEmailTaken    = errors.New("service: email already in use")
)

type inviteRepoIface interface {
	Create(ctx context.Context, i *model.Invite) error
	GetByToken(ctx context.Context, t string) (*model.Invite, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

type userRepoIface interface {
	GetByEmailGlobal(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
}

type InviteService struct {
	invites inviteRepoIface
	users   userRepoIface
	otps    otpRepo
	email   email.Sender
	audit   auditRepo
}

func NewInviteService(invs inviteRepoIface, users userRepoIface, otps otpRepo, em email.Sender, audit auditRepo) *InviteService {
	return &InviteService{invites: invs, users: users, otps: otps, email: em, audit: audit}
}

func (s *InviteService) GenerateForCenterAdmin(ctx context.Context, actorID, orgID uuid.UUID) (*model.Invite, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	inv := &model.Invite{
		OrganizationID: orgID,
		Role:           "center_admin",
		Token:          hex.EncodeToString(tokenBytes),
		ExpiresAt:      time.Now().Add(inviteTTL),
		CreatedBy:      actorID,
	}
	if err := s.invites.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}

	tt := "invite"
	tid := inv.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "generate_invite",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return inv, nil
}

// AcceptInvite validates the token, pre-creates the user with status=pending,
// marks the invite used, and sends an OTP. The user verifies via the existing
// /auth/otp/verify endpoint, which transitions pending → active.
func (s *InviteService) AcceptInvite(ctx context.Context, token, emailAddr, name string) error {
	inv, err := s.invites.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrInvalidInvite
		}
		return fmt.Errorf("lookup invite: %w", err)
	}
	if inv.UsedAt != nil {
		return ErrInvalidInvite
	}
	if time.Now().After(inv.ExpiresAt) {
		return ErrInvalidInvite
	}

	if _, err := s.users.GetByEmailGlobal(ctx, emailAddr); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return fmt.Errorf("check email: %w", err)
	}

	u := &model.User{
		Name:           name,
		Email:          &emailAddr,
		Role:           inv.Role,
		OrganizationID: &inv.OrganizationID,
		Status:         "pending",
		Language:       "ar",
	}
	if err := s.users.Create(ctx, u); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if err := s.invites.MarkUsed(ctx, inv.ID); err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}

	plain, hash, err := auth.GenerateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	if err := s.otps.Create(ctx, &model.OtpCode{
		Email:     emailAddr,
		Code:      hash,
		ExpiresAt: time.Now().Add(otpTTL),
	}); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}
	if err := s.email.Send(ctx, email.Message{
		To:      emailAddr,
		Subject: "Welcome to Mutqin — your login code",
		Body:    fmt.Sprintf("Welcome %s. Your code is %s. It expires in %d minutes.", name, plain, int(otpTTL.Minutes())),
	}); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run all service tests**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test ./internal/service/... -v 2>&1 | tail -25
```

Expected: all tests pass (existing 5 auth + 1 new auth + 3 org + 5 invite = 14 total).

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/service api/internal/repo/user.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "service: InviteService + AuthService activates pending users on first OTP verify"
```

---

### Task 8: Platform HTTP handlers

**Files:**
- Create: `api/internal/handler/api/platform.go`
- Create: `api/internal/handler/api/platform_test.go`

- [ ] **Step 1: Failing test**

```go
// api/internal/handler/api/platform_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubOrgService struct {
	createdInput service.CreateOrgInput
	createdActor uuid.UUID
	created      *model.Organization
	createErr    error
	listed       []model.Organization
}

func (s *stubOrgService) Create(_ context.Context, actor uuid.UUID, in service.CreateOrgInput) (*model.Organization, error) {
	s.createdInput = in
	s.createdActor = actor
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.created = &model.Organization{ID: uuid.New(), Name: in.Name, Slug: in.Slug, Country: in.Country, Tier: "free", Status: "active"}
	return s.created, nil
}
func (s *stubOrgService) GetBySlug(_ context.Context, slug string) (*model.Organization, error) {
	return nil, nil
}
func (s *stubOrgService) List(_ context.Context, limit, offset int) ([]model.Organization, error) {
	return s.listed, nil
}

type stubInviteService struct {
	generated    *model.Invite
	gotActor     uuid.UUID
	gotOrgID     uuid.UUID
	generateErr  error
}

func (s *stubInviteService) GenerateForCenterAdmin(_ context.Context, actor, orgID uuid.UUID) (*model.Invite, error) {
	s.gotActor = actor
	s.gotOrgID = orgID
	if s.generateErr != nil {
		return nil, s.generateErr
	}
	s.generated = &model.Invite{ID: uuid.New(), OrganizationID: orgID, Role: "center_admin", Token: "fake-token"}
	return s.generated, nil
}

func TestPlatform_CreateOrg_RequiresIdentity(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	body, _ := json.Marshal(map[string]string{"name": "X", "slug": "x", "country": "SO"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without identity, got %d", rec.Code)
	}
}

func TestPlatform_CreateOrg_Success(t *testing.T) {
	orgs := &stubOrgService{}
	h := apihandler.NewPlatformHandler(orgs, &stubInviteService{})

	body, _ := json.Marshal(map[string]string{"name": "Markaz", "slug": "markaz", "country": "SO"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations", bytes.NewReader(body))
	actor := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if orgs.createdInput.Name != "Markaz" {
		t.Fatalf("name: %s", orgs.createdInput.Name)
	}
	if orgs.createdActor != actor {
		t.Fatalf("actor mismatch")
	}
}

func TestPlatform_CreateOrg_BadJSON(t *testing.T) {
	h := apihandler.NewPlatformHandler(&stubOrgService{}, &stubInviteService{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("nope"))
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.CreateOrg(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestPlatform_GenerateInvite_Success(t *testing.T) {
	invs := &stubInviteService{}
	h := apihandler.NewPlatformHandler(&stubOrgService{}, invs)

	orgID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/"+orgID.String()+"/invite", nil)
	// Chi URL params won't be set without the router; use a context value the handler reads from.
	// Instead, we'll call the handler directly with a path that we'll parse manually.
	// Simpler: use a Chi RouteContext.
	import_chi(req, "id", orgID.String())
	actor := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	if invs.gotActor != actor {
		t.Fatalf("actor mismatch")
	}
	if invs.gotOrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.Token == "" {
		t.Fatal("token empty in response")
	}
}

// import_chi attaches a Chi URL param to the request context. Tests can't
// easily mount a full router; this helper sidesteps that.
func import_chi(r *http.Request, key, value string) {
	// see api/internal/handler/api/platform.go for the helper used in tests.
	apihandler.SetURLParamForTest(r, key, value)
}
```

- [ ] **Step 2: Implement (handler + tiny test helper for URL param injection)**

```go
// api/internal/handler/api/platform.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

type OrgService interface {
	Create(ctx context.Context, actor uuid.UUID, in service.CreateOrgInput) (*model.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	List(ctx context.Context, limit, offset int) ([]model.Organization, error)
}

type InviteIssuer interface {
	GenerateForCenterAdmin(ctx context.Context, actor, orgID uuid.UUID) (*model.Invite, error)
}

type PlatformHandler struct {
	orgs    OrgService
	invites InviteIssuer
}

func NewPlatformHandler(orgs OrgService, invites InviteIssuer) *PlatformHandler {
	return &PlatformHandler{orgs: orgs, invites: invites}
}

type createOrgBody struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Country     string  `json:"country"`
	City        *string `json:"city"`
	Description *string `json:"description"`
	Tier        string  `json:"tier"`
}

func (h *PlatformHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	var b createOrgBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	if strings.TrimSpace(b.Name) == "" || strings.TrimSpace(b.Slug) == "" || strings.TrimSpace(b.Country) == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "name, slug, country required")
		return
	}
	o, err := h.orgs.Create(r.Context(), id.UserID, service.CreateOrgInput{
		Name: b.Name, Slug: b.Slug, Country: b.Country, City: b.City, Description: b.Description, Tier: b.Tier,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not create organization")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": o})
}

func (h *PlatformHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.orgs.List(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

func (h *PlatformHandler) GetOrgBySlug(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	slug := chi.URLParam(r, "slug")
	o, err := h.orgs.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "organization not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "lookup failed")
		return
	}
	response.Success(w, o)
}

func (h *PlatformHandler) GenerateInvite(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	orgID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid org id")
		return
	}
	inv, err := h.invites.GenerateForCenterAdmin(r.Context(), id.UserID, orgID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not generate invite")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt,
	}})
}

// SetURLParamForTest is a thin testing helper — adds a Chi URL parameter to a
// request context so handlers that read chi.URLParam don't need a full router.
func SetURLParamForTest(r *http.Request, key, value string) {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	*r = *r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test ./internal/handler/api/... 2>&1 | tail -10
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/handler/api/platform.go api/internal/handler/api/platform_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "handler: platform (org create/list/get + invite generation)"
```

---

### Task 9: Invite-accept HTTP handler

**Files:**
- Create: `api/internal/handler/api/invite.go`
- Create: `api/internal/handler/api/invite_test.go`

- [ ] **Step 1: Failing tests**

```go
// api/internal/handler/api/invite_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubInviteAcceptor struct {
	gotToken string
	gotEmail string
	gotName  string
	err      error
}

func (s *stubInviteAcceptor) AcceptInvite(_ context.Context, token, em, name string) error {
	s.gotToken = token
	s.gotEmail = em
	s.gotName = name
	return s.err
}

func TestInviteAccept_HappyPath(t *testing.T) {
	stub := &stubInviteAcceptor{}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y.com", "name": "X Y"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invite/accept", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	if stub.gotToken != "tok" || stub.gotEmail != "x@y.com" || stub.gotName != "X Y" {
		t.Fatalf("got %s %s %s", stub.gotToken, stub.gotEmail, stub.gotName)
	}
}

func TestInviteAccept_BadInvite_Returns401(t *testing.T) {
	stub := &stubInviteAcceptor{err: service.ErrInvalidInvite}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestInviteAccept_EmailTaken_Returns409(t *testing.T) {
	stub := &stubInviteAcceptor{err: service.ErrEmailTaken}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestInviteAccept_OtherError_Returns500(t *testing.T) {
	stub := &stubInviteAcceptor{err: errors.New("db down")}
	h := apihandler.NewInviteAcceptHandler(stub)
	body, _ := json.Marshal(map[string]string{"token": "tok", "email": "x@y", "name": "X"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Accept(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/handler/api/invite.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

type InviteAcceptor interface {
	AcceptInvite(ctx context.Context, token, email, name string) error
}

type InviteAcceptHandler struct {
	svc InviteAcceptor
}

func NewInviteAcceptHandler(s InviteAcceptor) *InviteAcceptHandler {
	return &InviteAcceptHandler{svc: s}
}

type inviteAcceptBody struct {
	Token string `json:"token"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *InviteAcceptHandler) Accept(w http.ResponseWriter, r *http.Request) {
	var b inviteAcceptBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	b.Token = strings.TrimSpace(b.Token)
	b.Email = strings.TrimSpace(strings.ToLower(b.Email))
	b.Name = strings.TrimSpace(b.Name)
	if b.Token == "" || b.Email == "" || b.Name == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "token, email, name required")
		return
	}
	err := h.svc.AcceptInvite(r.Context(), b.Token, b.Email, b.Name)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInvite) {
			response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid invite")
			return
		}
		if errors.Is(err, service.ErrEmailTaken) {
			response.Error(w, http.StatusConflict, response.CodeConflict, "email already in use")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not accept invite")
		return
	}
	response.Success(w, map[string]string{"status": "code_sent"})
}
```

- [ ] **Step 3: Tests pass + commit**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go test ./internal/handler/api/... 2>&1 | tail -10
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/internal/handler/api/invite.go api/internal/handler/api/invite_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "handler: invite accept (POST /auth/invite/accept)"
```

---

### Task 10: Wire `cmd/server/main.go`

**Files:**
- Modify: `api/cmd/server/main.go`

- [ ] **Step 1: Add wiring**

After the `authH := apihandler.NewAuthHandler(authSvc)` line, add:

```go
	auditRepo := repo.NewAuditLogRepo(adminDB)
	orgSvc := service.NewOrganizationService(repo.NewOrganizationRepo(adminDB), auditRepo)
	inviteSvc := service.NewInviteService(
		repo.NewInviteRepo(adminDB),
		repo.NewUserRepo(adminDB),
		repo.NewOtpRepo(adminDB),
		emailSender,
		auditRepo,
	)
	platformH := apihandler.NewPlatformHandler(orgSvc, inviteSvc)
	inviteAcceptH := apihandler.NewInviteAcceptHandler(inviteSvc)
```

Replace the route block:

```go
	r.Get("/api/v1/health", handler.Health())
	r.Post("/api/v1/auth/otp/request", authH.RequestOTP)
	r.Post("/api/v1/auth/otp/verify", authH.VerifyOTP)
	r.Post("/api/v1/auth/invite/accept", inviteAcceptH.Accept)

	// Super-admin platform routes — Auth middleware already runs in the chain
	// above, so a valid JWT must be present. Role narrows to super_admin.
	r.Group(func(pr chi.Router) {
		pr.Use(middleware.Role("super_admin"))
		pr.Post("/api/v1/organizations", platformH.CreateOrg)
		pr.Get("/api/v1/organizations", platformH.ListOrgs)
		pr.Get("/api/v1/organizations/{slug}", platformH.GetOrgBySlug)
		pr.Post("/api/v1/organizations/{id}/invite", platformH.GenerateInvite)
	})
```

- [ ] **Step 2: Build + tests**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go build ./... && go test ./internal/handler/...
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i add api/cmd/server/main.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i commit -m "cmd: wire OrgService, InviteService, AuditLogRepo + super_admin route group"
```

---

### Task 11: End-to-end smoke

- [ ] **Step 1: Bring up the stack**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i
docker compose up -d --build db redis api traefik
sleep 12
```

- [ ] **Step 2: Bootstrap a super_admin and login**

```bash
cd api
DATABASE_URL='postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable' JWT_SECRET=dev-secret \
  go run ./cmd/bootstrap-admin --email=ops@mutqin.app --name=Ops 2>&1 | tail -2

curl -s -X POST http://localhost/api/v1/auth/otp/request -H 'Content-Type: application/json' -d '{"email":"ops@mutqin.app"}' >/dev/null
CODE=$(docker logs mutqin-api 2>&1 | grep -oE "Your code is [0-9]{6}" | tail -1 | grep -oE "[0-9]{6}")
TOKEN=$(curl -s -X POST http://localhost/api/v1/auth/otp/verify -H 'Content-Type: application/json' -d "{\"email\":\"ops@mutqin.app\",\"code\":\"$CODE\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['token'])")
echo "TOKEN=${TOKEN:0:30}..."
```

- [ ] **Step 3: Create an organization**

```bash
ORG_RESP=$(curl -s -X POST http://localhost/api/v1/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Markaz An-Noor","slug":"noor","country":"SO"}')
echo "$ORG_RESP"
ORG_ID=$(echo "$ORG_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['id'])")
echo "ORG_ID=$ORG_ID"
```

Expected: `data.id`, `data.slug=noor`, `data.tier=free`.

- [ ] **Step 4: Generate an invite for the org**

```bash
INVITE_RESP=$(curl -s -X POST "http://localhost/api/v1/organizations/$ORG_ID/invite" \
  -H "Authorization: Bearer $TOKEN")
echo "$INVITE_RESP"
INV_TOKEN=$(echo "$INVITE_RESP" | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['token'])")
echo "INV_TOKEN=${INV_TOKEN:0:20}..."
```

- [ ] **Step 5: Accept the invite as a new center admin**

```bash
curl -s -X POST http://localhost/api/v1/auth/invite/accept \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"$INV_TOKEN\",\"email\":\"admin@noor.so\",\"name\":\"Noor Admin\"}"
echo
sleep 1
NEW_CODE=$(docker logs mutqin-api 2>&1 | grep -oE "Your code is [0-9]{6}" | tail -1 | grep -oE "[0-9]{6}")
echo "NEW_CODE=$NEW_CODE"
```

- [ ] **Step 6: Verify OTP — should activate the pending user and return a JWT**

```bash
NEW_TOKEN=$(curl -s -X POST http://localhost/api/v1/auth/otp/verify \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"admin@noor.so\",\"code\":\"$NEW_CODE\"}" | python3 -c "import json,sys; print(json.load(sys.stdin)['data']['token'])")
echo "NEW_TOKEN=${NEW_TOKEN:0:30}..."
```

- [ ] **Step 7: Confirm the new center_admin user is active in the DB**

```bash
docker exec -i mutqin-db psql -U mutqin -d mutqin -c \
  "SELECT email, role, status, organization_id FROM users WHERE email = 'admin@noor.so';"
```

Expected: one row with `role=center_admin status=active organization_id=<ORG_ID>`.

- [ ] **Step 8: Confirm the audit log captured the create + invite events**

```bash
docker exec -i mutqin-db psql -U mutqin -d mutqin -c \
  "SELECT action, target_type FROM audit_log ORDER BY created_at;"
```

Expected: rows for `create_organization` and `generate_invite`.

- [ ] **Step 9: Tear down**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i && docker compose down
```

- [ ] **Step 10: Nothing to commit (verification only)**

---

### Task 12: Final verify + push

- [ ] **Step 1: Tests + vet**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i/api && go clean -testcache && go test ./... 2>&1 | tail -10
go vet ./...
```

- [ ] **Step 2: Push**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i push -u origin feat/plan-i-platform-admin
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-i push gitlab feat/plan-i-platform-admin
```

- [ ] **Step 3: Open PR + MR**

Base: `docs/backend-architecture-v2`. Title: `Plan I — Platform admin + invite flow`.

Body:

```
## Summary
- `audit_log` migration (global, not RLS-subject) + `AuditLog` model + `AuditLogRepo`.
- `InviteRepo` (Create / GetByToken / MarkUsed) and `UserRepo.Update` (status, name, role, language).
- `OrganizationService` (Create + audit log entry, GetBySlug, List).
- `InviteService` (GenerateForCenterAdmin + audit, AcceptInvite — pre-creates pending user + sends OTP + marks invite used).
- `AuthService.VerifyOTP` now activates pending users on first verify.
- `PlatformHandler` (POST /organizations, GET /organizations, GET /organizations/{slug}, POST /organizations/{id}/invite) — gated by `Auth` + `Role("super_admin")`.
- `InviteAcceptHandler` (POST /api/v1/auth/invite/accept) — public.

## Verification (e2e)
- [x] bootstrap-admin → login → create org → generate invite → accept invite as new email → OTP verify → new user is `center_admin status=active` in DB
- [x] audit_log shows both `create_organization` and `generate_invite` rows

## Out of scope (deferred)
- Suspend / tier-change endpoints (small follow-up)
- Platform-wide dashboard (Plan M)
- CF onboarding integration on org-create (operator runs `cmd/onboard` manually for now)
- Email-based invite delivery (out-of-band tokens for now)
```

---

## Self-Review

**PRD coverage (Epic 2):**
- FR1 (super admin creates org) → Tasks 6, 8 ✓
- FR2 (super admin generates invite for center admin) → Tasks 4, 7, 8 ✓
- FR3 (platform-wide dashboard) → out of scope (Plan M)
- FR4 (deactivate/suspend org) → small follow-up (Update endpoint shape exists via UserRepo.Update; org Update is similar)
- FR5 (view tiers) → list endpoint returns all fields including tier; UI is in `web/`

**FR21–24 partial coverage (Epic 1.2/1.3):**
- FR21 (teacher activates via invite + OTP) → AcceptInvite + VerifyOTP pending→active works for `center_admin` and `teacher` roles (only differs by what `inv.Role` is). Plan J extends with `GenerateForTeacher`.
- FR23 (RBAC) — Role middleware enforces; super-admin route group uses it.

**Placeholder scan:** None.

**Type / signature consistency:**
- `service.CreateOrgInput{Name, Slug, Country, City, Description, Tier}` consistent across Tasks 6 and 8.
- `OrgService.Create(ctx, actor, in)` matches handler call.
- `InviteIssuer.GenerateForCenterAdmin(ctx, actor, orgID)` matches handler call.
- `InviteAcceptor.AcceptInvite(ctx, token, email, name)` matches handler call.
- `service.ErrInvalidInvite`, `service.ErrEmailTaken` used in handler error mapping.
- `auth.From(ctx)` returns `Identity{UserID, OrgID, Role}`, used by handlers for actor + role checks.

**Scope:** 12 tasks. Substantive: 3, 4, 6, 7, 8, 9, 10. Trivial: 1, 2, 5, 11, 12. The `service` package gains 2 services + tests; `handler/api` gains 2 handlers + tests.
