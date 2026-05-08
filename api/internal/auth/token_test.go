// api/internal/auth/token_test.go
package auth_test

import (
	"strings"
	"testing"

	"github.com/ilyas/mutqin-api/internal/auth"
)

func TestGenerateOTP_Format(t *testing.T) {
	code, hash, err := auth.GenerateOTP()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("code len: got %d want 6", len(code))
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			t.Fatalf("non-digit char in code: %s", code)
		}
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("hash does not look like bcrypt: %s", hash)
	}
}

func TestVerifyOTPHash(t *testing.T) {
	code, hash, err := auth.GenerateOTP()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !auth.VerifyOTPHash(code, hash) {
		t.Fatal("correct code did not verify")
	}
	if auth.VerifyOTPHash("000000", hash) {
		t.Fatal("wrong code verified — hash collision or bug")
	}
}

func TestGenerateOTP_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		c, _, err := auth.GenerateOTP()
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		seen[c] = true
	}
	if len(seen) < 40 {
		t.Fatalf("only %d distinct codes out of 50 — randomness suspect", len(seen))
	}
}
