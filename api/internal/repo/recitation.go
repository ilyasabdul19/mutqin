// api/internal/repo/recitation.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/model"
)

type RecitationRepo struct {
	db bun.IDB
}

func NewRecitationRepo(d bun.IDB) *RecitationRepo {
	return &RecitationRepo{db: d}
}

func (r *RecitationRepo) idb(ctx context.Context) bun.IDB {
	return db.TxFrom(ctx, r.db)
}

func (r *RecitationRepo) Create(ctx context.Context, rec *model.Recitation) error {
	_, err := r.idb(ctx).NewInsert().Model(rec).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert recitation: %w", err)
	}
	return nil
}

func (r *RecitationRepo) BatchCreate(ctx context.Context, recs []model.Recitation) error {
	if len(recs) == 0 {
		return nil
	}
	_, err := r.idb(ctx).NewInsert().Model(&recs).Exec(ctx)
	if err != nil {
		return fmt.Errorf("batch insert recitations: %w", err)
	}
	return nil
}

func (r *RecitationRepo) ListByStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	var rows []model.Recitation
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("student_id = ?", studentID).
		OrderExpr("recorded_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list recitations by student: %w", err)
	}
	return rows, nil
}

func (r *RecitationRepo) GetLatestByStudent(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	rec := new(model.Recitation)
	err := r.idb(ctx).NewSelect().
		Model(rec).
		Where("student_id = ?", studentID).
		OrderExpr("recorded_at DESC").
		Limit(1).
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get latest recitation: %w", err)
	}
	return rec, nil
}

// ListByOrgSince returns recitations recorded after `since`, ascending by
// recorded_at. Tenant is applied via TenantScoped's BeforeSelect when ctx
// carries a tenant. Limit is clamped to (0, 500]; default 200.
func (r *RecitationRepo) ListByOrgSince(ctx context.Context, since time.Time, limit int) ([]model.Recitation, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []model.Recitation
	err := r.idb(ctx).NewSelect().
		Model(&rows).
		Where("recorded_at > ?", since).
		OrderExpr("recorded_at ASC").
		Limit(limit).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("recitations since: %w", err)
	}
	return rows, nil
}
