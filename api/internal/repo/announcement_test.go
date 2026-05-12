// api/internal/repo/announcement_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestAnnouncementRepo_CreateAndGet(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "A Org", Slug: "a-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewAnnouncementRepo(testAdmin)
	a := &model.Announcement{
		OrganizationID: org.ID,
		Title:          "Welcome",
		Body:           "Salaam.",
	}
	if err := r.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if a.ID == uuid.Nil {
		t.Fatal("expected ID populated after Create")
	}

	got, err := r.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Welcome" || got.Body != "Salaam." {
		t.Fatalf("got=%+v", got)
	}
	if got.OrganizationID != org.ID {
		t.Fatalf("org id mismatch")
	}
}

func TestAnnouncementRepo_GetByID_NotFound(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewAnnouncementRepo(testAdmin)
	if _, err := r.GetByID(ctx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestAnnouncementRepo_ListByOrgPublic_OrdersDescByCreatedAt(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "L Org", Slug: "l-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewAnnouncementRepo(testAdmin)
	for _, title := range []string{"first", "second", "third"} {
		a := &model.Announcement{OrganizationID: org.ID, Title: title, Body: "x"}
		if err := r.Create(ctx, a); err != nil {
			t.Fatalf("create %s: %v", title, err)
		}
	}

	got, err := r.ListByOrgPublic(ctx, org.ID, 10)
	if err != nil {
		t.Fatalf("ListByOrgPublic: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	// Most recent first (third created last).
	if got[0].Title != "third" {
		t.Fatalf("first row title=%s want third", got[0].Title)
	}
	if got[2].Title != "first" {
		t.Fatalf("last row title=%s want first", got[2].Title)
	}
}

func TestAnnouncementRepo_ListByOrgPublic_LimitsResults(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "Lim Org", Slug: "lim-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewAnnouncementRepo(testAdmin)
	for i := 0; i < 5; i++ {
		a := &model.Announcement{OrganizationID: org.ID, Title: "t", Body: "b"}
		if err := r.Create(ctx, a); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	got, err := r.ListByOrgPublic(ctx, org.ID, 3)
	if err != nil {
		t.Fatalf("ListByOrgPublic: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
}

func TestAnnouncementRepo_Delete(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "D Org", Slug: "d-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewAnnouncementRepo(testAdmin)
	a := &model.Announcement{OrganizationID: org.ID, Title: "doomed", Body: "x"}
	if err := r.Create(ctx, a); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := r.Delete(ctx, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.GetByID(ctx, a.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("post-delete GetByID err=%v want ErrNotFound", err)
	}
	if err := r.Delete(ctx, a.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("second Delete err=%v want ErrNotFound", err)
	}
}
