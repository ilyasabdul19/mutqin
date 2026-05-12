// api/internal/repo/registration_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

func TestRegistrationRepo_CreateAndList(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "Reg Org", Slug: "reg-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	regRepo := repo.NewRegistrationRepo(testAdmin)

	r := &model.Registration{
		OrganizationID: org.ID,
		ChildName:      "Mahmoud",
	}
	if err := regRepo.Create(ctx, r); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if r.ID == uuid.Nil {
		t.Fatal("expected ID populated after Create")
	}

	// Tenant-scoped list — TenantScoped hook narrows.
	got, err := regRepo.List(tenant.With(ctx, org.ID), 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 registration, got %d", len(got))
	}
	if got[0].ChildName != "Mahmoud" {
		t.Fatalf("want child_name=Mahmoud, got %s", got[0].ChildName)
	}
}

func TestRegistrationRepo_TenantNarrowsList(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "a", Country: "SO", Tier: "free", Status: "active"}
	orgB := &model.Organization{Name: "B", Slug: "b", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB)

	regRepo := repo.NewRegistrationRepo(testAdmin)
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgA.ID, ChildName: "A1"})
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgA.ID, ChildName: "A2"})
	_ = regRepo.Create(ctx, &model.Registration{OrganizationID: orgB.ID, ChildName: "B1"})

	gotA, err := regRepo.List(tenant.With(ctx, orgA.ID), 100, 0)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(gotA) != 2 {
		t.Fatalf("tenant A list: want 2, got %d", len(gotA))
	}

	gotB, err := regRepo.List(tenant.With(ctx, orgB.ID), 100, 0)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(gotB) != 1 {
		t.Fatalf("tenant B list: want 1, got %d", len(gotB))
	}
}
