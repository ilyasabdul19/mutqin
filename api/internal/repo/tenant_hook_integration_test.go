// api/internal/repo/tenant_hook_integration_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestTenantHook_NarrowsUserGetByID(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create org A: %v", err)
	}
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB); err != nil {
		t.Fatalf("create org B: %v", err)
	}

	userA := &model.User{Name: "User A", Email: ptrString("a@a"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, userA); err != nil {
		t.Fatalf("create user A: %v", err)
	}
	userB := &model.User{Name: "User B", Email: ptrString("b@b"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, userB); err != nil {
		t.Fatalf("create user B: %v", err)
	}

	userRepo := repo.NewUserRepo(testAdmin)
	ctxA := tenant.With(ctx, orgA.ID)
	if _, err := userRepo.GetByID(ctxA, userB.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("cross-tenant GetByID leaked: want ErrNotFound, got %v", err)
	}
	if _, err := userRepo.GetByID(ctxA, userA.ID); err != nil {
		t.Fatalf("same-tenant GetByID failed: %v", err)
	}
}

func TestTenantHook_NarrowsUserGetByEmail(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB)

	uA := &model.User{Name: "A", Email: ptrString("hook-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	uB := &model.User{Name: "B", Email: ptrString("hook-b@x"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uA)
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uB)

	userRepo := repo.NewUserRepo(testAdmin)
	ctxA := tenant.With(ctx, orgA.ID)
	if _, err := userRepo.GetByEmail(ctxA, "hook-b@x"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("cross-tenant GetByEmail leaked: want ErrNotFound, got %v", err)
	}
}

func TestTenantHook_OrganizationLookupUnaffected(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	org := &model.Organization{Name: "Z", Slug: "z", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	got, err := repo.NewOrganizationRepo(testAdmin).GetBySlug(ctx, "z")
	if err != nil {
		t.Fatalf("admin GetBySlug: %v", err)
	}
	if got.Slug != "z" {
		t.Fatalf("got slug %q, want z", got.Slug)
	}

	gotApp, err := repo.NewOrganizationRepo(testAdmin).GetBySlugAdmin(ctx, "z")
	if err != nil {
		t.Fatalf("hooked GetBySlugAdmin (no tenant ctx) failed: %v", err)
	}
	_ = gotApp

	// Place a tenant in ctx and query the organizations table. Organization
	// does not embed model.TenantScoped, so no per-model hook fires. If a
	// future maintainer mistakenly embeds TenantScoped on this table (which
	// has no organization_id column), Postgres would error with
	// `column "organization_id" does not exist` and this test would fail.
	uniqueOrg := uuid.New()
	ctxScoped := tenant.With(ctx, uniqueOrg)
	if _, err := repo.NewOrganizationRepo(testAdmin).GetBySlugAdmin(ctxScoped, "z"); err != nil {
		t.Fatalf("scoped lookup on organizations should not be hook-modified: %v", err)
	}
}
