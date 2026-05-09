// api/internal/service/invite.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

const inviteTTL = 72 * time.Hour

var (
	ErrInvalidInvite = errors.New("service: invalid invite")
	ErrEmailTaken    = errors.New("service: email already in use")
)

type inviteRepoIface interface {
	Create(ctx context.Context, i *model.Invite) error
	GetByToken(ctx context.Context, t string) (*model.Invite, error)
	MarkUsed(ctx context.Context, id uuid.UUID) error
}

type userRepoIface interface {
	GetByEmailGlobal(ctx context.Context, email string) (*model.User, error)
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
}

type InviteService struct {
	invites inviteRepoIface
	users   userRepoIface
	otps    otpRepo
	email   email.Sender
	audit   auditRepo
}

func NewInviteService(invs inviteRepoIface, users userRepoIface, otps otpRepo, em email.Sender, audit auditRepo) *InviteService {
	return &InviteService{invites: invs, users: users, otps: otps, email: em, audit: audit}
}

func (s *InviteService) GenerateForCenterAdmin(ctx context.Context, actorID, orgID uuid.UUID) (*model.Invite, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	inv := &model.Invite{
		OrganizationID: orgID,
		Role:           "center_admin",
		Token:          hex.EncodeToString(tokenBytes),
		ExpiresAt:      time.Now().Add(inviteTTL),
		CreatedBy:      actorID,
	}
	if err := s.invites.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create invite: %w", err)
	}

	tt := "invite"
	tid := inv.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "generate_invite",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return inv, nil
}

func (s *InviteService) GenerateForTeacher(ctx context.Context, actorID, orgID uuid.UUID) (*model.Invite, error) {
	tokenBytes := make([]byte, 24)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}
	inv := &model.Invite{
		OrganizationID: orgID,
		Role:           "teacher",
		Token:          hex.EncodeToString(tokenBytes),
		ExpiresAt:      time.Now().Add(inviteTTL),
		CreatedBy:      actorID,
	}
	if err := s.invites.Create(ctx, inv); err != nil {
		return nil, fmt.Errorf("create teacher invite: %w", err)
	}

	tt := "invite"
	tid := inv.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "generate_teacher_invite",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return inv, nil
}

// AcceptInvite validates the token, pre-creates the user with status=pending,
// marks the invite used, and sends an OTP. The user verifies via the existing
// /auth/otp/verify endpoint, which transitions pending → active.
func (s *InviteService) AcceptInvite(ctx context.Context, token, emailAddr, name string) error {
	inv, err := s.invites.GetByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return ErrInvalidInvite
		}
		return fmt.Errorf("lookup invite: %w", err)
	}
	if inv.UsedAt != nil {
		return ErrInvalidInvite
	}
	if time.Now().After(inv.ExpiresAt) {
		return ErrInvalidInvite
	}

	if _, err := s.users.GetByEmailGlobal(ctx, emailAddr); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, repo.ErrNotFound) {
		return fmt.Errorf("check email: %w", err)
	}

	u := &model.User{
		Name:           name,
		Email:          &emailAddr,
		Role:           inv.Role,
		OrganizationID: &inv.OrganizationID,
		Status:         "pending",
		Language:       "ar",
	}
	if err := s.users.Create(ctx, u); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if err := s.invites.MarkUsed(ctx, inv.ID); err != nil {
		return fmt.Errorf("mark invite used: %w", err)
	}

	plain, hash, err := auth.GenerateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	if err := s.otps.Create(ctx, &model.OtpCode{
		Email:     emailAddr,
		Code:      hash,
		ExpiresAt: time.Now().Add(otpTTL),
	}); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}
	if err := s.email.Send(ctx, email.Message{
		To:      emailAddr,
		Subject: "Welcome to Mutqin — your login code",
		Body:    fmt.Sprintf("Welcome %s. Your code is %s. It expires in %d minutes.", name, plain, int(otpTTL.Minutes())),
	}); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}
