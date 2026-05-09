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

func ptrString(s string) *string     { return &s }
func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }

func TestUserRepo_CreateAndGetByID(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	// users is RLS-subject; setup uses testAdmin (superuser bypass) so we
	// don't need a tenant in ctx just to seed fixtures.
	orgRepo := repo.NewOrganizationRepo(testAdmin)
	userRepo := repo.NewUserRepo(testAdmin)

	org := &model.Organization{Name: "Org A", Slug: "org-a", Country: "SO", Tier: "free", Status: "active"}
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
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	// users is RLS-subject; setup uses testAdmin (superuser bypass) so we
	// don't need a tenant in ctx just to seed fixtures.
	orgRepo := repo.NewOrganizationRepo(testAdmin)
	userRepo := repo.NewUserRepo(testAdmin)

	org := &model.Organization{Name: "Org B", Slug: "org-b", Country: "SO", Tier: "free", Status: "active"}
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
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	// No rows to find; admin handle keeps us out of RLS-empty-result territory.
	userRepo := repo.NewUserRepo(testAdmin)

	_, err := userRepo.GetByID(ctx, uuid.New())
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestUserRepo_GetByEmailGlobal_FindsAcrossTenants(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "global-a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create orgA: %v", err)
	}
	uA := &model.User{Name: "A", Email: ptrString("global-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uA); err != nil {
		t.Fatalf("create userA: %v", err)
	}

	got, err := repo.NewUserRepo(testAdmin).GetByEmailGlobal(ctx, "global-a@x")
	if err != nil {
		t.Fatalf("GetByEmailGlobal: %v", err)
	}
	if got.ID != uA.ID {
		t.Fatalf("got id %s want %s", got.ID, uA.ID)
	}
}

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
