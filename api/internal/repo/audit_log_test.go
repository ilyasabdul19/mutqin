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
