// api/internal/repo/sync_conflict_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestSyncConflictRepo_Create_PopulatesID(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	org := &model.Organization{Name: "SC Org", Slug: "sc-org", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, org); err != nil {
		t.Fatalf("create org: %v", err)
	}

	r := repo.NewSyncConflictRepo(testAdmin)
	cid := "client-xyz"
	sc := &model.SyncConflict{
		OrganizationID: org.ID,
		TableName:      "recitations",
		RecordID:       uuid.New(),
		ClientID:       &cid,
		ClientData:     []byte(`{"grade":"mumtaz"}`),
		ServerData:     []byte(`{"grade":"jayyid"}`),
	}
	if err := r.Create(ctx, sc); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sc.ID == uuid.Nil {
		t.Fatal("ID not populated after Create")
	}
	if sc.Resolution != "last_write_wins" {
		t.Fatalf("resolution=%s want last_write_wins (default)", sc.Resolution)
	}
}
