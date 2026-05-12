// api/internal/repo/rls_integration_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestRLS_BlocksCrossTenantSelect(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "rls-a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create org A: %v", err)
	}
	orgB := &model.Organization{Name: "B", Slug: "rls-b", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgB); err != nil {
		t.Fatalf("create org B: %v", err)
	}

	uA := &model.User{Name: "UA", Email: ptrString("rls-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	uB := &model.User{Name: "UB", Email: ptrString("rls-b@x"), Role: "teacher", OrganizationID: ptrUUID(orgB.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uA); err != nil {
		t.Fatalf("create user A: %v", err)
	}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uB); err != nil {
		t.Fatalf("create user B: %v", err)
	}

	// Run a transaction on the app handle, set tenant=A, count users.
	tx, err := testDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant = '"+orgA.ID.String()+"'"); err != nil {
		t.Fatalf("set local: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("RLS leaked: count=%d, want 1 (only org A's user)", count)
	}

	// Switching tenant to a random uuid yields zero rows.
	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant = '"+uuid.New().String()+"'"); err != nil {
		t.Fatalf("set local 2: %v", err)
	}
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count again: %v", err)
	}
	if count != 0 {
		t.Fatalf("RLS leaked: count=%d, want 0 for a random tenant", count)
	}
}

func TestRLS_NoTenantSet_ReturnsNoRows(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "rls-c", Country: "SO", Tier: "free", Status: "active"}
	_ = repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA)
	uA := &model.User{Name: "UA", Email: ptrString("rls-c@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	_ = repo.NewUserRepo(testAdmin).Create(ctx, uA)

	// app handle, no SET LOCAL — current_setting('app.current_tenant', true)
	// returns empty string; cast to uuid yields NULL; predicate fails => 0 rows.
	var count int
	if err := testDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("RLS without tenant should yield 0 rows, got %d", count)
	}
}
