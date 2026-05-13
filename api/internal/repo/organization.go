// api/internal/repo/organization.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type OrganizationRepo struct {
	db bun.IDB
}

func NewOrganizationRepo(db bun.IDB) *OrganizationRepo {
	return &OrganizationRepo{db: db}
}

func (r *OrganizationRepo) Create(ctx context.Context, org *model.Organization) error {
	_, err := r.db.NewInsert().Model(org).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert organization: %w", err)
	}
	return nil
}

func (r *OrganizationRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error) {
	org := new(model.Organization)
	err := r.db.NewSelect().Model(org).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select organization by id: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	org := new(model.Organization)
	err := r.db.NewSelect().Model(org).Where("slug = ?", slug).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select organization by slug: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) List(ctx context.Context, limit, offset int) ([]model.Organization, error) {
	var orgs []model.Organization
	err := r.db.NewSelect().
		Model(&orgs).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	return orgs, nil
}

// Update persists the landing-page-visible fields (description, logo_url,
// city, schedule). The caller already holds the org row (typically loaded
// via GetByID) so we just write back the mutable fields. Returns
// ErrNotFound when no row matches the id (e.g. concurrent delete).
func (r *OrganizationRepo) Update(ctx context.Context, org *model.Organization) error {
	q := r.db.NewUpdate().
		Model(org).
		Set("description = ?", org.Description).
		Set("logo_url = ?", org.LogoURL).
		Set("city = ?", org.City).
		Set("updated_at = now()")
	if len(org.Schedule) == 0 {
		q = q.Set("schedule = NULL")
	} else {
		// Cast the []byte to jsonb explicitly — bun encodes []byte as bytea
		// otherwise, which Postgres rejects for a jsonb column.
		q = q.Set("schedule = ?::jsonb", string(org.Schedule))
	}
	res, err := q.Where("id = ?", org.ID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("update organization: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetBySlugAdmin looks up an organization by slug WITHOUT a tenant in ctx.
// This is the entry point for the tenant-resolution chain: the request arrives,
// we extract the slug from Host or X-Tenant-Slug, and we need the UUID before
// we can populate the tenant ctx. Equivalent in behavior to GetBySlug; the
// distinct name flags it as an intentional cross-tenant call site.
func (r *OrganizationRepo) GetBySlugAdmin(ctx context.Context, slug string) (*model.Organization, error) {
	return r.GetBySlug(ctx, slug)
}
