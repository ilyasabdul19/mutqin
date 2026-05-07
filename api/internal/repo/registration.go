// api/internal/repo/registration.go
package repo

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type RegistrationRepo struct {
	db bun.IDB
}

func NewRegistrationRepo(db bun.IDB) *RegistrationRepo {
	return &RegistrationRepo{db: db}
}

func (r *RegistrationRepo) Create(ctx context.Context, reg *model.Registration) error {
	_, err := r.db.NewInsert().Model(reg).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert registration: %w", err)
	}
	return nil
}

func (r *RegistrationRepo) List(ctx context.Context, limit, offset int) ([]model.Registration, error) {
	var regs []model.Registration
	err := r.db.NewSelect().
		Model(&regs).
		OrderExpr("submitted_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list registrations: %w", err)
	}
	return regs, nil
}
