// api/internal/repo/organization_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestOrganizationRepo_CreateAndGet(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	org := &model.Organization{
		Name:    "Markaz Al-Falah",
		Slug:    "al-falah",
		Country: "SO",
		Tier:    "free",
		Status:  "active",
	}
	if err := r.Create(ctx, org); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if org.ID == uuid.Nil {
		t.Fatal("expected ID populated after Create")
	}

	got, err := r.GetByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Slug != "al-falah" {
		t.Fatalf("want slug al-falah, got %s", got.Slug)
	}
}

func TestOrganizationRepo_GetBySlug(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	org := &model.Organization{
		Name:    "Markaz Noor",
		Slug:    "noor",
		Country: "SO",
		Tier:    "free",
		Status:  "active",
	}
	if err := r.Create(ctx, org); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.GetBySlug(ctx, "noor")
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.ID != org.ID {
		t.Fatalf("want ID %s, got %s", org.ID, got.ID)
	}
}

func TestOrganizationRepo_GetByID_NotFound(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	_, err := r.GetByID(ctx, uuid.New())
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestOrganizationRepo_List(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOrganizationRepo(testDB)

	for _, slug := range []string{"list-a", "list-b", "list-c"} {
		org := &model.Organization{
			Name:    "List Org " + slug,
			Slug:    slug,
			Country: "SO",
			Tier:    "free",
			Status:  "active",
		}
		if err := r.Create(ctx, org); err != nil {
			t.Fatalf("Create %s: %v", slug, err)
		}
	}

	got, err := r.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want exactly 3 orgs after isolated test, got %d", len(got))
	}
}
