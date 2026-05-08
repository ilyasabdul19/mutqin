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
