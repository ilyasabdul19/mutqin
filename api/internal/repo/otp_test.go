// api/internal/repo/otp_test.go
package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func TestOtpRepo_CreateAndConsume(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOtpRepo(testAdmin)

	code := &model.OtpCode{
		Email:     "user@example.com",
		Code:      "$2a$10$fakehash",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	if err := r.Create(ctx, code); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := r.ConsumeActive(ctx, "user@example.com")
	if err != nil {
		t.Fatalf("ConsumeActive: %v", err)
	}
	if got.ID != code.ID {
		t.Fatalf("ID mismatch")
	}
	// Second consume should return ErrNotFound.
	if _, err := r.ConsumeActive(ctx, "user@example.com"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("second consume: want ErrNotFound, got %v", err)
	}
}

func TestOtpRepo_ConsumeRejectsExpired(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOtpRepo(testAdmin)

	expired := &model.OtpCode{
		Email:     "exp@example.com",
		Code:      "$2a$10$fakehash",
		ExpiresAt: time.Now().Add(-time.Minute), // already expired
	}
	if err := r.Create(ctx, expired); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := r.ConsumeActive(ctx, "exp@example.com"); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("expired consume: want ErrNotFound, got %v", err)
	}
}

func TestOtpRepo_PartialIndexBlocksDoubleActive(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()
	r := repo.NewOtpRepo(testAdmin)

	first := &model.OtpCode{Email: "x@x", Code: "$2a$10$h1", ExpiresAt: time.Now().Add(time.Minute)}
	if err := r.Create(ctx, first); err != nil {
		t.Fatalf("first create: %v", err)
	}
	second := &model.OtpCode{Email: "x@x", Code: "$2a$10$h2", ExpiresAt: time.Now().Add(time.Minute)}
	if err := r.Create(ctx, second); err == nil {
		t.Fatal("second create on same email with used=false: want unique-violation, got nil")
	}
}
