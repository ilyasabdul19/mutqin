// api/internal/service/auth_test.go
package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/service"
)

// --- mock OtpRepo ---
type stubOtpRepo struct {
	created    *model.OtpCode
	consumed   *model.OtpCode
	consumeErr error
}

func (s *stubOtpRepo) Create(_ context.Context, c *model.OtpCode) error {
	c.ID = uuid.New()
	s.created = c
	return nil
}

func (s *stubOtpRepo) ConsumeActive(_ context.Context, _ string) (*model.OtpCode, error) {
	if s.consumeErr != nil {
		return nil, s.consumeErr
	}
	if s.consumed == nil {
		return nil, repo.ErrNotFound
	}
	return s.consumed, nil
}

// --- mock UserRepo ---
type stubUserRepo struct {
	users map[string]*model.User
}

func (s *stubUserRepo) GetByEmailGlobal(_ context.Context, email string) (*model.User, error) {
	u, ok := s.users[email]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return u, nil
}

func (s *stubUserRepo) Update(_ context.Context, u *model.User) error {
	if existing, ok := s.users[*u.Email]; ok && existing.ID == u.ID {
		s.users[*u.Email] = u
		return nil
	}
	return repo.ErrNotFound
}

// --- tests ---

func TestAuthService_RequestOTP_NewCodeStoredAndEmailed(t *testing.T) {
	otps := &stubOtpRepo{}
	users := &stubUserRepo{users: map[string]*model.User{
		"user@x": {ID: uuid.New(), Email: ptrStr("user@x"), Role: "teacher", OrganizationID: ptrUUID(uuid.New())},
	}}
	es := email.NewLogSender()
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")

	svc := service.NewAuthService(otps, users, es, iss, time.Hour)

	if err := svc.RequestOTP(context.Background(), "user@x"); err != nil {
		t.Fatalf("RequestOTP: %v", err)
	}
	if otps.created == nil {
		t.Fatal("OTP not stored")
	}
	if otps.created.Email != "user@x" {
		t.Fatalf("stored email: %s", otps.created.Email)
	}
	if es.Last().To != "user@x" {
		t.Fatalf("email To: %s", es.Last().To)
	}
}

func TestAuthService_RequestOTP_UnknownEmailDoesNotLeak(t *testing.T) {
	otps := &stubOtpRepo{}
	users := &stubUserRepo{users: map[string]*model.User{}}
	es := email.NewLogSender()
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")

	svc := service.NewAuthService(otps, users, es, iss, time.Hour)
	// Should NOT return error — to avoid email enumeration. No OTP stored.
	if err := svc.RequestOTP(context.Background(), "ghost@x"); err != nil {
		t.Fatalf("RequestOTP unknown email: want nil, got %v", err)
	}
	if otps.created != nil {
		t.Fatal("OTP should not be stored for unknown email")
	}
	if es.Last().To != "" {
		t.Fatalf("email should not have been sent, but To=%s", es.Last().To)
	}
}

func TestAuthService_VerifyOTP_HappyPath_ReturnsToken(t *testing.T) {
	uid := uuid.New()
	orgID := uuid.New()
	users := &stubUserRepo{users: map[string]*model.User{
		"u@x": {ID: uid, Email: ptrStr("u@x"), Role: "center_admin", OrganizationID: ptrUUID(orgID)},
	}}

	plain, hash, _ := auth.GenerateOTP()
	otps := &stubOtpRepo{consumed: &model.OtpCode{Email: "u@x", Code: hash}}

	es := email.NewLogSender()
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")

	svc := service.NewAuthService(otps, users, es, iss, time.Hour)

	tok, err := svc.VerifyOTP(context.Background(), "u@x", plain)
	if err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	got, err := ver.Verify(tok)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if got.UserID != uid {
		t.Fatalf("user_id: %s", got.UserID)
	}
	if got.OrgID == nil || *got.OrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if got.Role != "center_admin" {
		t.Fatalf("role: %s", got.Role)
	}
}

func TestAuthService_VerifyOTP_WrongCode(t *testing.T) {
	users := &stubUserRepo{users: map[string]*model.User{
		"u@x": {ID: uuid.New(), Email: ptrStr("u@x"), Role: "teacher", OrganizationID: ptrUUID(uuid.New())},
	}}
	_, hash, _ := auth.GenerateOTP()
	otps := &stubOtpRepo{consumed: &model.OtpCode{Email: "u@x", Code: hash}}

	svc := service.NewAuthService(otps, users, email.NewLogSender(), auth.NewIssuer([]byte("s"), "mutqin-api"), time.Hour)
	if _, err := svc.VerifyOTP(context.Background(), "u@x", "000000"); err == nil {
		t.Fatal("want error on wrong code")
	}
}

func TestAuthService_VerifyOTP_NoActiveCode(t *testing.T) {
	otps := &stubOtpRepo{consumeErr: repo.ErrNotFound}
	users := &stubUserRepo{}
	svc := service.NewAuthService(otps, users, email.NewLogSender(), auth.NewIssuer([]byte("s"), "mutqin-api"), time.Hour)
	if _, err := svc.VerifyOTP(context.Background(), "u@x", "123456"); !errors.Is(err, service.ErrInvalidOTP) {
		t.Fatalf("want ErrInvalidOTP, got %v", err)
	}
}

func TestAuthService_VerifyOTP_ActivatesPendingUser(t *testing.T) {
	uid := uuid.New()
	orgID := uuid.New()
	users := &stubUserRepo{users: map[string]*model.User{
		"u@x": {ID: uid, Email: ptrStr("u@x"), Role: "center_admin", OrganizationID: ptrUUID(orgID), Status: "pending"},
	}}
	plain, hash, _ := auth.GenerateOTP()
	otps := &stubOtpRepo{consumed: &model.OtpCode{Email: "u@x", Code: hash}}
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	svc := service.NewAuthService(otps, users, email.NewLogSender(), iss, time.Hour)

	if _, err := svc.VerifyOTP(context.Background(), "u@x", plain); err != nil {
		t.Fatalf("VerifyOTP: %v", err)
	}
	if users.users["u@x"].Status != "active" {
		t.Fatalf("status=%s want active", users.users["u@x"].Status)
	}
}

func ptrStr(s string) *string         { return &s }
func ptrUUID(u uuid.UUID) *uuid.UUID  { return &u }
