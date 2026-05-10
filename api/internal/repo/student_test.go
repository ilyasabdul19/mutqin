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
