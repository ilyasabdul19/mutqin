// api/internal/repo/invite.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type InviteRepo struct {
	db bun.IDB
}

func NewInviteRepo(db bun.IDB) *InviteRepo {
	return &InviteRepo{db: db}
}

func (r *InviteRepo) Create(ctx context.Context, inv *model.Invite) error {
	_, err := r.db.NewInsert().Model(inv).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert invite: %w", err)
	}
	return nil
}

// GetByToken looks up an invite by token without a tenant filter — invites are
// looked up before the tenant ctx is known. Caller passes the admin handle.
func (r *InviteRepo) GetByToken(ctx context.Context, token string) (*model.Invite, error) {
	inv := new(model.Invite)
	err := r.db.NewSelect().Model(inv).Where("token = ?", token).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select invite: %w", err)
	}
	return inv, nil
}

func (r *InviteRepo) MarkUsed(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.NewUpdate().
		Model((*model.Invite)(nil)).
		Set("used_at = ?", now).
		Where("id = ?", id).
		Where("used_at IS NULL").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}
	return nil
}
