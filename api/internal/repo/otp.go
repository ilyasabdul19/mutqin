// api/internal/repo/otp.go
package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/model"
)

type OtpRepo struct {
	db bun.IDB
}

func NewOtpRepo(db bun.IDB) *OtpRepo {
	return &OtpRepo{db: db}
}

func (r *OtpRepo) Create(ctx context.Context, code *model.OtpCode) error {
	_, err := r.db.NewInsert().Model(code).Returning("*").Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert otp: %w", err)
	}
	return nil
}

// ConsumeActive marks the active (used=false, not-expired) OTP for the given
// email as used and returns it. Returns ErrNotFound if no active code exists.
//
// Atomic via UPDATE ... RETURNING in a single round trip.
func (r *OtpRepo) ConsumeActive(ctx context.Context, email string) (*model.OtpCode, error) {
	got := new(model.OtpCode)
	err := r.db.NewUpdate().
		Model(got).
		Set("used = ?", true).
		Where("email = ?", email).
		Where("used = ?", false).
		Where("expires_at > ?", time.Now()).
		Returning("*").
		Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("consume otp: %w", err)
	}
	return got, nil
}
