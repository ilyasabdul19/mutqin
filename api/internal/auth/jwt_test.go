// api/internal/auth/jwt_test.go
package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
)

func TestJWT_RoundTrip(t *testing.T) {
	iss := auth.NewIssuer([]byte("test-secret"), "mutqin-api")
	ver := auth.NewVerifier([]byte("test-secret"), "mutqin-api")

	uid := uuid.New()
	orgID := uuid.New()
	tok, err := iss.Sign(auth.Claims{
		UserID: uid,
		OrgID:  &orgID,
		Role:   "teacher",
		TTL:    time.Hour,
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if !strings.Contains(tok, ".") {
		t.Fatalf("token shape unexpected: %s", tok)
	}

	got, err := ver.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.UserID != uid {
		t.Fatalf("user_id: got %s want %s", got.UserID, uid)
	}
	if got.OrgID == nil || *got.OrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if got.Role != "teacher" {
		t.Fatalf("role: got %s want teacher", got.Role)
	}
}

func TestJWT_Expired(t *testing.T) {
	iss := auth.NewIssuer([]byte("test-secret"), "mutqin-api")
	ver := auth.NewVerifier([]byte("test-secret"), "mutqin-api")

	tok, err := iss.Sign(auth.Claims{
		UserID: uuid.New(),
		Role:   "teacher",
		TTL:    -time.Minute, // already expired
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ver.Verify(tok); err == nil {
		t.Fatal("want expiry error")
	}
}

func TestJWT_BadSignature(t *testing.T) {
	iss := auth.NewIssuer([]byte("test-secret"), "mutqin-api")
	verWrong := auth.NewVerifier([]byte("WRONG-secret"), "mutqin-api")

	tok, err := iss.Sign(auth.Claims{UserID: uuid.New(), Role: "teacher", TTL: time.Hour})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := verWrong.Verify(tok); err == nil {
		t.Fatal("want signature error")
	}
}

func TestJWT_WrongIssuer(t *testing.T) {
	iss := auth.NewIssuer([]byte("test-secret"), "mutqin-api")
	verOther := auth.NewVerifier([]byte("test-secret"), "other-issuer")

	tok, _ := iss.Sign(auth.Claims{UserID: uuid.New(), Role: "teacher", TTL: time.Hour})
	if _, err := verOther.Verify(tok); err == nil {
		t.Fatal("want issuer error")
	}
}

func TestJWT_SuperAdminWithoutOrg(t *testing.T) {
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")

	uid := uuid.New()
	tok, err := iss.Sign(auth.Claims{UserID: uid, OrgID: nil, Role: "super_admin", TTL: time.Hour})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	got, err := ver.Verify(tok)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.OrgID != nil {
		t.Fatalf("org_id: got %v want nil", got.OrgID)
	}
}
