// api/internal/db/hooks_test.go
package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/schema"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/tenant"
)

type tenantTestRow struct {
	bun.BaseModel  `bun:"table:tenant_test_rows,alias:t"`
	ID             uuid.UUID `bun:",pk,type:uuid"`
	OrganizationID uuid.UUID `bun:"organization_id,type:uuid,notnull"`
}

func rendererBunDB() *bun.DB {
	return bun.NewDB(nil, pgdialect.New())
}

func TestTenantHook_InjectsWhere_OnSelect(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewSelect().Model((*tenantTestRow)(nil))
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewQueryGen(pgdialect.New()), nil)
	if !strings.Contains(string(sql), `"organization_id" =`) {
		t.Fatalf("SELECT did not get organization_id filter, sql=%q", string(sql))
	}
}

func TestTenantHook_NoCtx_NoFilter(t *testing.T) {
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewSelect().Model((*tenantTestRow)(nil))
	hook.BeforeQuery(context.Background(), &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewQueryGen(pgdialect.New()), nil)
	if strings.Contains(string(sql), `"organization_id" =`) || strings.Contains(string(sql), "WHERE") {
		t.Fatalf("expected no organization_id filter on empty ctx, got %q", string(sql))
	}
}

func TestTenantHook_Update(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewUpdate().Model(&tenantTestRow{ID: uuid.New()}).Set("organization_id = ?", uuid.New()).Where("id = ?", uuid.New())
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewQueryGen(pgdialect.New()), nil)
	if !strings.Contains(string(sql), `"organization_id" =`) {
		t.Fatalf("UPDATE did not get organization_id filter, sql=%q", string(sql))
	}
}

func TestTenantHook_Delete(t *testing.T) {
	orgID := uuid.New()
	ctx := tenant.With(context.Background(), orgID)
	hook := db.TenantHook{}
	bdb := rendererBunDB()
	q := bdb.NewDelete().Model((*tenantTestRow)(nil)).Where("id = ?", uuid.New())
	hook.BeforeQuery(ctx, &bun.QueryEvent{IQuery: q})
	sql, _ := q.AppendQuery(schema.NewQueryGen(pgdialect.New()), nil)
	if !strings.Contains(string(sql), `"organization_id" =`) {
		t.Fatalf("DELETE did not get organization_id filter, sql=%q", string(sql))
	}
}
