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
