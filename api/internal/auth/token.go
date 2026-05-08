// api/internal/auth/token.go
package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

const otpDigits = 6

// GenerateOTP returns (plaintext 6-digit code, bcrypt hash of code, error).
// Plaintext is sent to the user (email). Hash is what we persist; the plaintext
// is gone after this call.
func GenerateOTP() (string, string, error) {
	max := big.NewInt(1)
	for i := 0; i < otpDigits; i++ {
		max.Mul(max, big.NewInt(10))
	}
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", "", fmt.Errorf("rand: %w", err)
	}
	code := fmt.Sprintf("%0*d", otpDigits, n.Int64())

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", "", fmt.Errorf("hash: %w", err)
	}
	return code, string(hash), nil
}

// VerifyOTPHash returns true iff plaintext matches the bcrypt hash. Constant
// time within bcrypt itself.
func VerifyOTPHash(code, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}
