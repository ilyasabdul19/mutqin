// api/internal/repo/announcement.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type AnnouncementRepo struct {
	db bun.IDB
}

func NewAnnouncementRepo(d bun.IDB) *AnnouncementRepo {
	return &AnnouncementRepo{db: d}
}

// idb returns the request-scoped tx from ctx if RLSContext middleware has set
// one (so SET LOCAL app.current_tenant applies); otherwise returns the handle
// the repo was constructed with. Required for RLS-subject tables.
func (r *AnnouncementRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

func (r *AnnouncementRepo) Create(ctx context.Context, a *model.Announcement) error {
	_, err := r.idb(ctx).NewInsert().Model(a).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert announcement: %w", err)
	}
	return nil
}

func (r *AnnouncementRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Announcement, error) {
	a := new(model.Announcement)
	err := r.idb(ctx).NewSelect().Model(a).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select announcement: %w", err)
	}
	return a, nil
}

func (r *AnnouncementRepo) ListByOrg(ctx context.Context, limit, offset int) ([]model.Announcement, error) {
	var rows []model.Announcement
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list announcements: %w", err)
	}
	return rows, nil
}

// ListByOrgPublic is the cross-tenant read path used by the public landing
// handler. The landing handler resolves slug → org via the admin handle and
// has no tenant ctx, so it cannot rely on TenantScoped's BeforeSelect filter.
// Instead, we bypass the per-request tx (use r.db directly) and pass orgID
// explicitly in WHERE — the same intentional cross-tenant shape used by
// GetBySlugAdmin on OrganizationRepo.
func (r *AnnouncementRepo) ListByOrgPublic(ctx context.Context, orgID uuid.UUID, limit int) ([]model.Announcement, error) {
	var rows []model.Announcement
	err := r.db.NewSelect().
		Model(&rows).
		Where("organization_id = ?", orgID).
		OrderExpr("created_at DESC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list announcements public: %w", err)
	}
	return rows, nil
}

func (r *AnnouncementRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res, err := r.idb(ctx).NewDelete().
		Model((*model.Announcement)(nil)).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
