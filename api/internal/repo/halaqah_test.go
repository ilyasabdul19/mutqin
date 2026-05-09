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
