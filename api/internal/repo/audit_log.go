// api/internal/repo/audit_log.go
package repo

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type AuditLogRepo struct {
	db bun.IDB
}

func NewAuditLogRepo(db bun.IDB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, row *model.AuditLog) error {
	_, err := r.db.NewInsert().Model(row).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert audit_log: %w", err)
	}
	return nil
}

func (r *AuditLogRepo) ListByActor(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error) {
	var rows []model.AuditLog
	err := r.db.NewSelect().
		Model(&rows).
		Where("actor_id = ?", actorID).
		OrderExpr("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list audit_log: %w", err)
	}
	return rows, nil
}
