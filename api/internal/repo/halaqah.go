// api/internal/repo/halaqah.go
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

type HalaqahRepo struct {
	db bun.IDB
}

func NewHalaqahRepo(db bun.IDB) *HalaqahRepo {
	return &HalaqahRepo{db: db}
}

func (r *HalaqahRepo) Create(ctx context.Context, h *model.Halaqah) error {
	_, err := r.db.NewInsert().Model(h).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert halaqah: %w", err)
	}
	return nil
}

func (r *HalaqahRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error) {
	h := new(model.Halaqah)
	err := r.db.NewSelect().Model(h).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select halaqah: %w", err)
	}
	return h, nil
}

func (r *HalaqahRepo) ListByOrg(ctx context.Context, limit, offset int) ([]model.Halaqah, error) {
	var rows []model.Halaqah
	err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list halaqat: %w", err)
	}
	return rows, nil
}

func (r *HalaqahRepo) Update(ctx context.Context, h *model.Halaqah) error {
	res, err := r.db.NewUpdate().
		Model(h).
		Set("name = ?", h.Name).
		Set("teacher_id = ?", h.TeacherID).
		Set("max_capacity = ?", h.MaxCapacity).
		Set("status = ?", h.Status).
		Where("id = ?", h.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update halaqah: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
