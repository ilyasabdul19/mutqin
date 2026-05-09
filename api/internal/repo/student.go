// api/internal/repo/student.go
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

type StudentRepo struct {
	db bun.IDB
}

func NewStudentRepo(db bun.IDB) *StudentRepo {
	return &StudentRepo{db: db}
}

func (r *StudentRepo) Create(ctx context.Context, s *model.Student) error {
	_, err := r.db.NewInsert().Model(s).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert student: %w", err)
	}
	return nil
}

func (r *StudentRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Student, error) {
	s := new(model.Student)
	err := r.db.NewSelect().Model(s).Where("id = ?", id).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select student: %w", err)
	}
	return s, nil
}

func (r *StudentRepo) ListByHalaqah(ctx context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error) {
	var rows []model.Student
	err := r.db.NewSelect().
		Model(&rows).
		Where("halaqah_id = ?", halaqahID).
		OrderExpr("name ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list students by halaqah: %w", err)
	}
	return rows, nil
}

func (r *StudentRepo) ListByOrg(ctx context.Context, limit, offset int) ([]model.Student, error) {
	var rows []model.Student
	err := r.db.NewSelect().
		Model(&rows).
		OrderExpr("name ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list students: %w", err)
	}
	return rows, nil
}

func (r *StudentRepo) Update(ctx context.Context, s *model.Student) error {
	res, err := r.db.NewUpdate().
		Model(s).
		Set("name = ?", s.Name).
		Set("age = ?", s.Age).
		Set("parent_phone = ?", s.ParentPhone).
		Set("parent_email = ?", s.ParentEmail).
		Set("hifz_level = ?", s.HifzLevel).
		Set("status = ?", s.Status).
		Where("id = ?", s.ID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("update student: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *StudentRepo) Transfer(ctx context.Context, studentID, newHalaqahID uuid.UUID) error {
	res, err := r.db.NewUpdate().
		Model((*model.Student)(nil)).
		Set("halaqah_id = ?", newHalaqahID).
		Where("id = ?", studentID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("transfer student: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
