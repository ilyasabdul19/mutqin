// api/internal/service/auth.go
package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

const otpTTL = 5 * time.Minute

// ErrInvalidOTP is returned when the supplied OTP code does not match an
// active row in otp_codes (expired, already consumed, wrong code).
var ErrInvalidOTP = errors.New("service: invalid OTP")

// otpRepo is the slice of OtpRepo this service uses; defining the interface
// here lets us inject a mock without coupling tests to the full repo type.
type otpRepo interface {
	Create(ctx context.Context, c *model.OtpCode) error
	ConsumeActive(ctx context.Context, email string) (*model.OtpCode, error)
}

type userRepo interface {
	GetByEmailGlobal(ctx context.Context, email string) (*model.User, error)
}

type AuthService struct {
	otps  otpRepo
	users userRepo
	email email.Sender
	jwt   *auth.Issuer
	ttl   time.Duration
}

func NewAuthService(otps otpRepo, users userRepo, em email.Sender, iss *auth.Issuer, jwtTTL time.Duration) *AuthService {
	return &AuthService{otps: otps, users: users, email: em, jwt: iss, ttl: jwtTTL}
}

// RequestOTP generates a code, stores its hash, and emails the plaintext to
// the user. Unknown emails return nil error to prevent enumeration.
func (s *AuthService) RequestOTP(ctx context.Context, emailAddr string) error {
	if _, err := s.users.GetByEmailGlobal(ctx, emailAddr); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil // no leak
		}
		return fmt.Errorf("lookup user: %w", err)
	}

	plain, hash, err := auth.GenerateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	rec := &model.OtpCode{
		Email:     emailAddr,
		Code:      hash,
		ExpiresAt: time.Now().Add(otpTTL),
	}
	if err := s.otps.Create(ctx, rec); err != nil {
		return fmt.Errorf("store otp: %w", err)
	}

	if err := s.email.Send(ctx, email.Message{
		To:      emailAddr,
		Subject: "Your Mutqin login code",
		Body:    fmt.Sprintf("Your code is %s. It expires in %d minutes.", plain, int(otpTTL.Minutes())),
	}); err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

// VerifyOTP atomically consumes the OTP and, on success, returns a signed JWT.
func (s *AuthService) VerifyOTP(ctx context.Context, emailAddr, code string) (string, error) {
	row, err := s.otps.ConsumeActive(ctx, emailAddr)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return "", ErrInvalidOTP
		}
		return "", fmt.Errorf("consume otp: %w", err)
	}
	if !auth.VerifyOTPHash(code, row.Code) {
		return "", ErrInvalidOTP
	}

	u, err := s.users.GetByEmailGlobal(ctx, emailAddr)
	if err != nil {
		return "", fmt.Errorf("lookup user: %w", err)
	}

	tok, err := s.jwt.Sign(auth.Claims{
		UserID: u.ID,
		OrgID:  u.OrgID(),
		Role:   u.Role,
		TTL:    s.ttl,
	})
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return tok, nil
}
