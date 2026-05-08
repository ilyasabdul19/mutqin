// api/internal/auth/jwt.go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims is the application-level JWT payload. UserID is required; OrgID is
// nullable for super_admins (who do not belong to an organization).
type Claims struct {
	UserID uuid.UUID
	OrgID  *uuid.UUID
	Role   string
	TTL    time.Duration
}

type registeredClaims struct {
	UserID uuid.UUID `json:"sub"`
	OrgID  string    `json:"org,omitempty"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

// Issuer signs JWTs with HS256 + a shared secret.
type Issuer struct {
	secret []byte
	issuer string
}

func NewIssuer(secret []byte, issuer string) *Issuer {
	return &Issuer{secret: secret, issuer: issuer}
}

func (s *Issuer) Sign(c Claims) (string, error) {
	if c.UserID == uuid.Nil {
		return "", errors.New("auth: UserID required")
	}
	now := time.Now()
	rc := registeredClaims{
		UserID: c.UserID,
		Role:   c.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(c.TTL)),
		},
	}
	if c.OrgID != nil {
		rc.OrgID = c.OrgID.String()
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, rc)
	signed, err := tok.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("sign: %w", err)
	}
	return signed, nil
}

// Verifier validates HS256 JWTs.
type Verifier struct {
	secret []byte
	issuer string
}

func NewVerifier(secret []byte, issuer string) *Verifier {
	return &Verifier{secret: secret, issuer: issuer}
}

func (v *Verifier) Verify(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(
		token,
		&registeredClaims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return v.secret, nil
		},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(v.issuer),
	)
	if err != nil {
		return nil, err
	}
	rc, ok := parsed.Claims.(*registeredClaims)
	if !ok || !parsed.Valid {
		return nil, errors.New("auth: invalid claims")
	}
	out := &Claims{
		UserID: rc.UserID,
		Role:   rc.Role,
	}
	if rc.OrgID != "" {
		id, err := uuid.Parse(rc.OrgID)
		if err != nil {
			return nil, fmt.Errorf("auth: bad org claim: %w", err)
		}
		out.OrgID = &id
	}
	return out, nil
}
