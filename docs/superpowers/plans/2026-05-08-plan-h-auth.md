# Plan H — Auth (Email OTP + JWT + RBAC)

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the auth foundation — email-OTP login, HS256 JWT issuance/verification, AuthMiddleware that populates request ctx with user/org/role, and RoleMiddleware. Add a `bootstrap-admin` CLI to seed the first super-admin user (proper invite flow lands in Plan I).

**Architecture:** A new `internal/auth` package owns JWT signing+verifying and OTP code generation. A new `internal/email` package exposes an interface with a slog-based stub default implementation. `internal/service` is introduced (first feature service) and holds `AuthService` orchestrating OTP request + verify. `internal/handler/api` splits up: existing health stays, new `auth.go` handler module mounts `/api/v1/auth/*`. Middleware grows: `Auth` extracts the JWT and populates ctx via existing `tenant.With` plus a new `auth.With` carrier; `Role` checks the resolved role against a list.

**Refresh-token deliberately deferred.** JWT TTL is 24h. When refresh becomes important (cross-device, paranoid revocation) it lands in a small follow-up plan with a `refresh_tokens` table.

**Tech Stack:**
- New: `github.com/golang-jwt/jwt/v5` (HS256), `crypto/rand`, `crypto/subtle`, `golang.org/x/crypto/bcrypt` (OTP code hashing at rest)
- Existing: Bun, Chi, slog, testcontainers

**Out of scope (deferred):**
- Invite flow (Plan I — Super Admin generates invite, recipient verifies OTP + creates user)
- Refresh tokens / refresh endpoint / cookie-based refresh
- Real SMTP send (Plan H ships a stub; real Resend integration is a small follow-up gated on `RESEND_API_KEY`)
- Phone OTP (decision in arch.md: email only at MVP)
- Audit log (Plan I)

---

## File Structure

### Created

| Path | Responsibility |
|------|----------------|
| `api/internal/auth/jwt.go` | `Issuer.Sign(claims)` and `Verifier.Verify(token)` for HS256 |
| `api/internal/auth/jwt_test.go` | Round-trip, expiry, bad signature, wrong issuer |
| `api/internal/auth/token.go` | `GenerateOTP() (string, hash, error)` + `VerifyOTPHash(code, hash)` using bcrypt |
| `api/internal/auth/token_test.go` | OTP gen returns 6 digits, hash verifies, wrong code rejected |
| `api/internal/auth/context.go` | ctx carrier for `Identity{UserID, OrgID, Role}` |
| `api/internal/email/sender.go` | `Sender` interface + `LogSender` stub (slog-based) |
| `api/internal/email/sender_test.go` | LogSender records calls |
| `api/internal/repo/otp.go` | `OtpRepo` — Create, ConsumeActive (atomic) |
| `api/internal/repo/otp_test.go` | Integration tests against testcontainer |
| `api/internal/repo/user_lookup.go` | `OrganizationRepo` already has `GetBySlugAdmin`; add `UserRepo.GetByEmailGlobal` for cross-tenant login lookup |
| `api/internal/service/auth.go` | `AuthService` — RequestOTP, VerifyOTP. Orchestrates OtpRepo + UserRepo + email + JWT. |
| `api/internal/service/auth_test.go` | Service tests with mocks for repos + email |
| `api/internal/handler/api/auth.go` | HTTP handlers: `POST /auth/otp/request`, `POST /auth/otp/verify` |
| `api/internal/handler/api/auth_test.go` | Handler tests with mocked AuthService |
| `api/internal/middleware/auth_jwt.go` | `Auth` middleware — extracts Bearer token, validates, populates ctx |
| `api/internal/middleware/auth_jwt_test.go` | Tests for present/absent/invalid token |
| `api/internal/middleware/role.go` | `Role(allowed ...string)` — checks ctx role, 403 on mismatch |
| `api/internal/middleware/role_test.go` | Tests for allow/deny |
| `api/cmd/bootstrap-admin/main.go` | One-shot CLI: creates a `super_admin` user with given `--email` |
| `api/internal/migrate/migrations/20260508000001_otp_codes_email_index.go` | Adds `WHERE used = false` partial unique index on `otp_codes(email)` to prevent multiple active codes |

### Modified

| Path | Change |
|------|--------|
| `api/internal/config/config.go` | Add `JWTIssuer string` (defaults `mutqin-api`); the existing `JWTSecret` is reused |
| `api/cmd/server/main.go` | Wire `AuthService` + auth handlers + `Auth` middleware (currently mounted but not enforced — applied per-route in later plans) |
| `api/go.mod` | Add `github.com/golang-jwt/jwt/v5`, `golang.org/x/crypto` |

### Deleted
None.

---

## Tasks

### Task 1: Add deps

**Files:**
- Modify: `api/go.mod`, `api/go.sum`

- [ ] **Step 1: go get**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && \
  go get \
    github.com/golang-jwt/jwt/v5 \
    golang.org/x/crypto/bcrypt
```

- [ ] **Step 2: tidy + build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go mod tidy && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/go.mod api/go.sum
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "deps: add jwt/v5 + x/crypto/bcrypt for auth"
```

---

### Task 2: JWT issuer + verifier (TDD)

**Files:**
- Create: `api/internal/auth/jwt.go`
- Create: `api/internal/auth/jwt_test.go`

- [ ] **Step 1: Failing test**

```go
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
```

- [ ] **Step 2: Implement**

```go
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
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/auth/...
```

- [ ] **Step 4: Commit (red + green together for this small unit)**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/auth/jwt.go api/internal/auth/jwt_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "auth: HS256 JWT issuer + verifier"
```

---

### Task 3: OTP code generator (TDD)

**Files:**
- Create: `api/internal/auth/token.go`
- Create: `api/internal/auth/token_test.go`

- [ ] **Step 1: Failing test**

```go
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
```

- [ ] **Step 2: Implement**

```go
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
```

- [ ] **Step 3: Run tests, expect PASS**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/auth/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/auth/token.go api/internal/auth/token_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "auth: OTP generator + bcrypt hash"
```

---

### Task 4: Auth ctx carrier

**Files:**
- Create: `api/internal/auth/context.go`

- [ ] **Step 1: Write file**

```go
// api/internal/auth/context.go
package auth

import (
	"context"

	"github.com/google/uuid"
)

// Identity is what AuthMiddleware extracts from the JWT and places in ctx.
type Identity struct {
	UserID uuid.UUID
	OrgID  *uuid.UUID // nil for super_admin
	Role   string     // "super_admin" | "center_admin" | "teacher"
}

type ctxKey struct{}

// With returns a child ctx carrying the resolved Identity.
func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// From returns the Identity from ctx and ok=true if present.
func From(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(ctxKey{}).(Identity)
	return v, ok
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go build ./internal/auth
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/auth/context.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "auth: Identity ctx carrier"
```

---

### Task 5: Email sender (interface + log stub) (TDD)

**Files:**
- Create: `api/internal/email/sender.go`
- Create: `api/internal/email/sender_test.go`

- [ ] **Step 1: Failing tests**

```go
// api/internal/email/sender_test.go
package email_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ilyas/mutqin-api/internal/email"
)

func TestLogSender_Records(t *testing.T) {
	s := email.NewLogSender()
	if err := s.Send(context.Background(), email.Message{
		To:      "user@example.com",
		Subject: "Code",
		Body:    "Your code is 123456",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	last := s.Last()
	if last.To != "user@example.com" {
		t.Fatalf("to: got %s", last.To)
	}
	if !strings.Contains(last.Body, "123456") {
		t.Fatalf("body: %s", last.Body)
	}
}

func TestLogSender_RejectsEmptyTo(t *testing.T) {
	s := email.NewLogSender()
	err := s.Send(context.Background(), email.Message{To: "", Subject: "x", Body: "y"})
	if err == nil {
		t.Fatal("want error on empty To")
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/email/sender.go
//
// Email delivery interface. Plan H ships a slog-based stub (LogSender) for
// dev + tests. A real SMTP/Resend implementation lands in a follow-up plan
// gated on RESEND_API_KEY. Until then, OTP codes appear in the api server logs.
package email

import (
	"context"
	"errors"
	"log/slog"
	"sync"
)

// Message is a single email to send.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender delivers emails. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// LogSender writes the message to slog at INFO level and records the last
// message for tests to inspect.
type LogSender struct {
	mu   sync.Mutex
	last Message
}

func NewLogSender() *LogSender { return &LogSender{} }

func (s *LogSender) Send(ctx context.Context, m Message) error {
	if m.To == "" {
		return errors.New("email: empty To")
	}
	s.mu.Lock()
	s.last = m
	s.mu.Unlock()
	slog.InfoContext(ctx, "email send (LogSender)",
		"to", m.To, "subject", m.Subject, "body", m.Body)
	return nil
}

func (s *LogSender) Last() Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/email/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/email
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "email: Sender interface + LogSender stub"
```

---

### Task 6: Migration — partial unique index on otp_codes

**Files:**
- Create: `api/internal/migrate/migrations/20260508000001_otp_codes_email_index.go`

This prevents two active codes for the same email at once (so verify is unambiguous).

- [ ] **Step 1: Write migration**

```go
// api/internal/migrate/migrations/20260508000001_otp_codes_email_index.go
package migrations

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/ilyas/mutqin-api/internal/migrate"
)

func init() {
	migrate.Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx,
			`CREATE UNIQUE INDEX idx_otp_codes_email_active
			   ON otp_codes(email) WHERE used = false`)
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx,
			`DROP INDEX IF EXISTS idx_otp_codes_email_active`)
		return err
	})
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go build ./internal/migrate/migrations
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/migrate/migrations/20260508000001_otp_codes_email_index.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "migrate: partial unique index on otp_codes(email) WHERE used=false"
```

---

### Task 7: OtpRepo (TDD)

**Files:**
- Create: `api/internal/repo/otp.go`
- Create: `api/internal/repo/otp_test.go`

- [ ] **Step 1: Failing tests**

```go
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
```

- [ ] **Step 2: Implement**

```go
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
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test -run TestOtp ./internal/repo/... -v
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/repo/otp.go api/internal/repo/otp_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "repo: OtpRepo (Create + atomic ConsumeActive)"
```

---

### Task 8: `UserRepo.GetByEmailGlobal` (cross-tenant lookup for login)

**Files:**
- Modify: `api/internal/repo/user.go`
- Modify: `api/internal/repo/user_test.go` — add a new test using `testAdmin`

- [ ] **Step 1: Append the method**

Open `api/internal/repo/user.go` and add after the existing `GetByEmail` method:

```go
// GetByEmailGlobal looks up a user by email WITHOUT a tenant in ctx — used by
// the login flow to find the user before the tenant is known. Caller must use
// the admin handle (testAdmin in tests, adminDB in main).
func (r *UserRepo) GetByEmailGlobal(ctx context.Context, email string) (*model.User, error) {
	u := new(model.User)
	err := r.db.NewSelect().Model(u).Where("email = ?", email).Scan(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select user by email (global): %w", err)
	}
	return u, nil
}
```

- [ ] **Step 2: Add the test**

Open `api/internal/repo/user_test.go` and append a new function:

```go
func TestUserRepo_GetByEmailGlobal_FindsAcrossTenants(t *testing.T) {
	t.Cleanup(func() { truncateAll(t) })
	ctx := context.Background()

	orgA := &model.Organization{Name: "A", Slug: "global-a", Country: "SO", Tier: "free", Status: "active"}
	if err := repo.NewOrganizationRepo(testAdmin).Create(ctx, orgA); err != nil {
		t.Fatalf("create orgA: %v", err)
	}
	uA := &model.User{Name: "A", Email: ptrString("global-a@x"), Role: "teacher", OrganizationID: ptrUUID(orgA.ID), Status: "active", Language: "ar"}
	if err := repo.NewUserRepo(testAdmin).Create(ctx, uA); err != nil {
		t.Fatalf("create userA: %v", err)
	}

	got, err := repo.NewUserRepo(testAdmin).GetByEmailGlobal(ctx, "global-a@x")
	if err != nil {
		t.Fatalf("GetByEmailGlobal: %v", err)
	}
	if got.ID != uA.ID {
		t.Fatalf("got id %s want %s", got.ID, uA.ID)
	}
}
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test -run TestUserRepo ./internal/repo/... -v 2>&1 | tail -10
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/repo/user.go api/internal/repo/user_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "repo: UserRepo.GetByEmailGlobal for cross-tenant login"
```

---

### Task 9: `AuthService` (TDD with mocks)

**Files:**
- Create: `api/internal/service/auth.go`
- Create: `api/internal/service/auth_test.go`

- [ ] **Step 1: Write failing tests**

```go
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
	created  *model.OtpCode
	consumed *model.OtpCode
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

func ptrStr(s string) *string { return &s }
func ptrUUID(u uuid.UUID) *uuid.UUID { return &u }
```

- [ ] **Step 2: Implement**

```go
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
	otps   otpRepo
	users  userRepo
	email  email.Sender
	jwt    *auth.Issuer
	ttl    time.Duration
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
```

> Note: `u.OrgID()` is a method on `*model.User` — a small helper because `model.User.OrganizationID` is `*uuid.UUID`. Add the helper if it doesn't exist:
>
> ```go
> // in api/internal/model/user.go
> // OrgID returns the user's organization id pointer (nil for super_admin).
> func (u *User) OrgID() *uuid.UUID { return u.OrganizationID }
> ```
>
> Or skip the helper and use `u.OrganizationID` directly in the Sign call. Either works.

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/service/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/service api/internal/model/user.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "service: AuthService (RequestOTP, VerifyOTP) — first service-layer file"
```

---

### Task 10: Auth HTTP handler (TDD)

**Files:**
- Create: `api/internal/handler/api/auth.go`
- Create: `api/internal/handler/api/auth_test.go`

- [ ] **Step 1: Failing tests**

```go
// api/internal/handler/api/auth_test.go
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/service"
)

type stubAuthService struct {
	requestErr error
	verifyTok  string
	verifyErr  error
	gotEmail   string
	gotCode    string
}

func (s *stubAuthService) RequestOTP(ctx context.Context, email string) error {
	s.gotEmail = email
	return s.requestErr
}

func (s *stubAuthService) VerifyOTP(ctx context.Context, email, code string) (string, error) {
	s.gotEmail = email
	s.gotCode = code
	return s.verifyTok, s.verifyErr
}

func TestAuthHandler_RequestOTP_AcceptsValidEmail(t *testing.T) {
	stub := &stubAuthService{}
	h := apihandler.NewAuthHandler(stub)

	body, _ := json.Marshal(map[string]string{"email": "user@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.RequestOTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d", rec.Code)
	}
	if stub.gotEmail != "user@example.com" {
		t.Fatalf("email: got %q", stub.gotEmail)
	}
}

func TestAuthHandler_RequestOTP_RejectsBadJSON(t *testing.T) {
	h := apihandler.NewAuthHandler(&stubAuthService{})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	h.RequestOTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d", rec.Code)
	}
}

func TestAuthHandler_VerifyOTP_ReturnsTokenOnSuccess(t *testing.T) {
	stub := &stubAuthService{verifyTok: "the-jwt"}
	h := apihandler.NewAuthHandler(stub)

	body, _ := json.Marshal(map[string]string{"email": "u@x", "code": "123456"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/otp/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d (body=%s)", rec.Code, rec.Body.String())
	}

	var resp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Data.Token != "the-jwt" {
		t.Fatalf("token: %s", resp.Data.Token)
	}
}

func TestAuthHandler_VerifyOTP_ReturnsUnauthorizedOnInvalid(t *testing.T) {
	stub := &stubAuthService{verifyErr: service.ErrInvalidOTP}
	h := apihandler.NewAuthHandler(stub)
	body, _ := json.Marshal(map[string]string{"email": "u@x", "code": "999999"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", rec.Code)
	}
}

func TestAuthHandler_VerifyOTP_ReturnsInternalOnOtherError(t *testing.T) {
	stub := &stubAuthService{verifyErr: errors.New("db down")}
	h := apihandler.NewAuthHandler(stub)
	body, _ := json.Marshal(map[string]string{"email": "u@x", "code": "999999"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.VerifyOTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/handler/api/auth.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// AuthService is the surface AuthHandler needs. Production implementation is
// *service.AuthService; tests provide a stub.
type AuthService interface {
	RequestOTP(ctx context.Context, email string) error
	VerifyOTP(ctx context.Context, email, code string) (string, error)
}

type AuthHandler struct {
	svc AuthService
}

func NewAuthHandler(s AuthService) *AuthHandler { return &AuthHandler{svc: s} }

type otpRequestBody struct {
	Email string `json:"email"`
}

func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var body otpRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid request body")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if body.Email == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "email is required")
		return
	}
	if err := h.svc.RequestOTP(r.Context(), body.Email); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not request code")
		return
	}
	response.Success(w, map[string]string{"status": "sent"})
}

type otpVerifyBody struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body otpVerifyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid request body")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Code = strings.TrimSpace(body.Code)
	if body.Email == "" || body.Code == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "email and code are required")
		return
	}

	tok, err := h.svc.VerifyOTP(r.Context(), body.Email, body.Code)
	if err != nil {
		if errors.Is(err, service.ErrInvalidOTP) {
			response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid code")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not verify code")
		return
	}
	response.Success(w, map[string]string{"token": tok})
}
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/handler/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/handler/api
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "handler: auth HTTP handlers (request + verify OTP)"
```

---

### Task 11: AuthMiddleware (TDD)

**Files:**
- Create: `api/internal/middleware/auth_jwt.go`
- Create: `api/internal/middleware/auth_jwt_test.go`

- [ ] **Step 1: Failing tests**

```go
// api/internal/middleware/auth_jwt_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestAuth_PopulatesIdentity(t *testing.T) {
	iss := auth.NewIssuer([]byte("s"), "mutqin-api")
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)

	uid := uuid.New()
	orgID := uuid.New()
	tok, _ := iss.Sign(auth.Claims{UserID: uid, OrgID: &orgID, Role: "teacher", TTL: time.Minute})

	var got auth.Identity
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok = auth.From(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !ok {
		t.Fatal("identity not set")
	}
	if got.UserID != uid {
		t.Fatalf("user_id: %s", got.UserID)
	}
	if got.OrgID == nil || *got.OrgID != orgID {
		t.Fatalf("org_id mismatch")
	}
	if got.Role != "teacher" {
		t.Fatalf("role: %s", got.Role)
	}
}

func TestAuth_NoHeader_PassesThroughWithoutIdentity(t *testing.T) {
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)
	var ok bool
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok = auth.From(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if ok {
		t.Fatal("identity should not be present without Authorization header")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
}

func TestAuth_BadToken_Returns401(t *testing.T) {
	ver := auth.NewVerifier([]byte("s"), "mutqin-api")
	mw := middleware.Auth(ver)
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", rec.Code)
	}
	if called {
		t.Fatal("handler should not have run")
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/middleware/auth_jwt.go
package middleware

import (
	"net/http"
	"strings"

	"github.com/ilyas/mutqin-api/internal/auth"
)

// Auth returns middleware that, when an Authorization: Bearer <jwt> header is
// present, validates the token via Verifier and stores the resulting Identity
// in ctx via auth.With. Bad tokens → 401. Absent header → pass through (so
// public routes work). Routes that require auth must compose Role(...) after
// Auth or check auth.From() themselves.
func Auth(v *auth.Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !strings.HasPrefix(h, "Bearer ") {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			tok := strings.TrimPrefix(h, "Bearer ")
			claims, err := v.Verify(tok)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := auth.With(r.Context(), auth.Identity{
				UserID: claims.UserID,
				OrgID:  claims.OrgID,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/middleware/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/middleware/auth_jwt.go api/internal/middleware/auth_jwt_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "middleware: Auth (JWT Bearer → Identity in ctx)"
```

---

### Task 12: RoleMiddleware (TDD)

**Files:**
- Create: `api/internal/middleware/role.go`
- Create: `api/internal/middleware/role_test.go`

- [ ] **Step 1: Failing tests**

```go
// api/internal/middleware/role_test.go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/middleware"
)

func TestRole_AllowsMatchingRole(t *testing.T) {
	mw := middleware.Role("center_admin", "super_admin")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	ctx := auth.With(httptest.NewRequest(http.MethodGet, "/", nil).Context(), auth.Identity{
		UserID: uuid.New(), Role: "center_admin",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("matching role: handler should have been called")
	}
}

func TestRole_DeniesOtherRole(t *testing.T) {
	mw := middleware.Role("super_admin")
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	ctx := auth.With(httptest.NewRequest(http.MethodGet, "/", nil).Context(), auth.Identity{
		UserID: uuid.New(), Role: "teacher",
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Fatal("non-matching role: handler should not have been called")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d want 403", rec.Code)
	}
}

func TestRole_NoIdentity_Returns401(t *testing.T) {
	mw := middleware.Role("teacher")
	h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler should not run")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status: got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Implement**

```go
// api/internal/middleware/role.go
package middleware

import (
	"net/http"

	"github.com/ilyas/mutqin-api/internal/auth"
)

// Role returns middleware that requires the request's Identity to have a role
// in the allowed list. Missing identity → 401. Wrong role → 403. Compose AFTER
// the Auth middleware.
func Role(allowed ...string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		allowedSet[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.From(r.Context())
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			if _, ok := allowedSet[id.Role]; !ok {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Tests pass**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./internal/middleware/...
```

- [ ] **Step 4: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/middleware/role.go api/internal/middleware/role_test.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "middleware: Role (RBAC check on ctx Identity)"
```

---

### Task 13: Bootstrap CLI — `cmd/bootstrap-admin`

**Files:**
- Create: `api/cmd/bootstrap-admin/main.go`

A one-shot CLI to create the first super admin so the OTP login flow can be exercised before Plan I lands the invite system.

- [ ] **Step 1: Write the CLI**

```go
// api/cmd/bootstrap-admin/main.go
//
// bootstrap-admin creates a single super_admin user so the system has a way in
// before Plan I ships the proper invite flow.
//
// Usage:
//   bootstrap-admin --email=ops@mutqin.app --name="Ops"
//
// Idempotent: if a user with that email already exists, prints its id and exits 0.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"

	"github.com/ilyas/mutqin-api/internal/config"
	"github.com/ilyas/mutqin-api/internal/db"
	"github.com/ilyas/mutqin-api/internal/migrate"
	_ "github.com/ilyas/mutqin-api/internal/migrate/migrations"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	emailFlag := flag.String("email", "", "super admin email (required)")
	nameFlag := flag.String("name", "Super Admin", "display name")
	flag.Parse()

	if *emailFlag == "" {
		slog.Error("--email is required")
		flag.Usage()
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	adminDB, err := db.NewDB(ctx, cfg.DatabaseURL, false)
	if err != nil {
		slog.Error("connect db", "error", err)
		os.Exit(1)
	}
	defer adminDB.Close()

	if err := migrate.Up(ctx, adminDB); err != nil {
		slog.Error("migrate", "error", err)
		os.Exit(1)
	}

	users := repo.NewUserRepo(adminDB)

	if existing, err := users.GetByEmailGlobal(ctx, *emailFlag); err == nil {
		slog.Info("user already exists", "id", existing.ID, "email", *emailFlag)
		return
	} else if !errors.Is(err, repo.ErrNotFound) {
		slog.Error("lookup", "error", err)
		os.Exit(1)
	}

	u := &model.User{
		Name:     *nameFlag,
		Email:    emailFlag,
		Role:     "super_admin",
		Status:   "active",
		Language: "ar",
	}
	if err := users.Create(ctx, u); err != nil {
		slog.Error("create user", "error", err)
		os.Exit(1)
	}
	slog.Info("super_admin created", "id", u.ID, "email", *emailFlag)
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go build ./cmd/bootstrap-admin
```

- [ ] **Step 3: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/cmd/bootstrap-admin/main.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "cmd: bootstrap-admin CLI (creates first super_admin)"
```

---

### Task 14: Wire `cmd/server/main.go`

**Files:**
- Modify: `api/internal/config/config.go` — add `JWTIssuer string`
- Modify: `api/cmd/server/main.go` — wire AuthService + auth handlers + Auth middleware

- [ ] **Step 1: Config addition**

In `api/internal/config/config.go`:

Add field to `Config` struct:

```go
	JWTIssuer string
```

In `Load()`, after the existing JWT block:

```go
	cfg.JWTIssuer = os.Getenv("JWT_ISSUER")
	if cfg.JWTIssuer == "" {
		cfg.JWTIssuer = "mutqin-api"
	}
```

- [ ] **Step 2: Update `main.go`**

In `api/cmd/server/main.go`, after the existing handler/repo wiring (after `orgLookup := orgLookupAdapter{...}`) and before `r := chi.NewRouter()`, add:

```go
	// Auth wiring.
	jwtIssuer := auth.NewIssuer([]byte(cfg.JWTSecret), cfg.JWTIssuer)
	jwtVerifier := auth.NewVerifier([]byte(cfg.JWTSecret), cfg.JWTIssuer)
	emailSender := email.NewLogSender() // swap to SMTP when RESEND_API_KEY lands
	authSvc := service.NewAuthService(
		repo.NewOtpRepo(adminDB),
		repo.NewUserRepo(adminDB),
		emailSender,
		jwtIssuer,
		24*time.Hour,
	)
	authH := apihandler.NewAuthHandler(authSvc)
```

Add imports near the top:

```go
	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/email"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/service"
```

Replace the existing single route registration with:

```go
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger(logger))
	r.Use(middleware.CORS)
	r.Use(middleware.Tenant(orgLookup, cfg.BaseHost))
	r.Use(middleware.Auth(jwtVerifier))
	r.Use(middleware.RLSContext(middleware.NewBunRunner(appDB)))

	r.Get("/api/v1/health", handler.Health())
	r.Post("/api/v1/auth/otp/request", authH.RequestOTP)
	r.Post("/api/v1/auth/otp/verify", authH.VerifyOTP)
```

- [ ] **Step 3: Verify build**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go build ./...
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go test ./...
```

- [ ] **Step 5: Commit**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h add api/internal/config/config.go api/cmd/server/main.go
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h commit -m "cmd: wire AuthService + auth handlers + Auth middleware"
```

---

### Task 15: End-to-end smoke

- [ ] **Step 1: Bring up stack**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h
docker compose up -d --build api db redis traefik
sleep 10
```

- [ ] **Step 2: Bootstrap a super_admin from inside the api container**

```bash
docker compose exec api /app/server --help 2>&1 | head -3 # sanity
# bootstrap-admin needs its own image OR run via go run from outside
cd api && DATABASE_URL='postgres://mutqin:mutqin@localhost:5432/mutqin?sslmode=disable' \
  JWT_SECRET=dev-secret \
  go run ./cmd/bootstrap-admin --email=ops@mutqin.app --name=Ops
```

Expected: `super_admin created id=<uuid>`.

- [ ] **Step 3: Request an OTP**

```bash
curl -s -i -X POST http://localhost/api/v1/auth/otp/request \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@mutqin.app"}'
```

Expected: 200 `{"data":{"status":"sent"}}`.

- [ ] **Step 4: Read the code from the api container's logs**

```bash
docker logs mutqin-api 2>&1 | grep -E "email send|body" | tail -2
```

Expected: a log line with `body=...your code is 123456...`.

- [ ] **Step 5: Verify the OTP**

```bash
curl -s -i -X POST http://localhost/api/v1/auth/otp/verify \
  -H 'Content-Type: application/json' \
  -d '{"email":"ops@mutqin.app","code":"<the-6-digit-code>"}'
```

Expected: 200 with `{"data":{"token":"<jwt>"}}`.

- [ ] **Step 6: Smoke the JWT**

```bash
curl -s -i http://localhost/api/v1/health \
  -H "Authorization: Bearer <the-jwt>"
```

Expected: 200 (handler unaffected by auth, but verify the middleware doesn't reject a valid token).

```bash
curl -s -i http://localhost/api/v1/health \
  -H "Authorization: Bearer not-a-real-token"
```

Expected: 401.

- [ ] **Step 7: Tear down**

```bash
docker compose down
```

- [ ] **Step 8: Nothing to commit (verification only)**

---

### Task 16: Final verify + push

- [ ] **Step 1: Tests + vet**

```bash
cd /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h/api && go clean -testcache && go test ./... 2>&1 | tail -10
go vet ./...
```

- [ ] **Step 2: Push**

```bash
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h push -u origin feat/plan-h-auth
git -C /Users/suleimanmayow/Desktop/Projects/mutqin-plan-h push gitlab feat/plan-h-auth
```

- [ ] **Step 3: PR + MR**

Base: `docs/backend-architecture-v2`.

Title: `Plan H — Auth foundation (Email OTP + JWT + RBAC)`

Body:

```
## Summary
- `internal/auth` — HS256 JWT issuer/verifier; OTP generator (6-digit, bcrypt hash); Identity ctx carrier.
- `internal/email` — Sender interface + LogSender stub (slog). Real SMTP/Resend gated on RESEND_API_KEY in a follow-up.
- `internal/repo/OtpRepo` with atomic `ConsumeActive` + partial unique index migration.
- `UserRepo.GetByEmailGlobal` for cross-tenant login.
- `internal/service/AuthService` — first service-layer file. Orchestrates OTP request + verify.
- `internal/handler/api/AuthHandler` — `POST /api/v1/auth/otp/{request,verify}`. Validation errors → 400, invalid OTP → 401, infra errors → 500.
- `middleware.Auth` (JWT → Identity) + `middleware.Role(allowed...)` (RBAC).
- `cmd/bootstrap-admin` CLI to seed the first super_admin until Plan I ships the invite flow.

## Verification
- [x] All unit + integration tests pass.
- [x] End-to-end smoke: bootstrap admin → request OTP → read code from logs → verify → JWT issued → /health accepts token, rejects garbage.

## Out of scope (deferred)
- Real SMTP send (RESEND_API_KEY-gated)
- Refresh tokens / cookie-based refresh
- Invite flow (Plan I)
- Audit log (Plan I)
```

---

## Self-Review

**Spec coverage:**
- Auth flow from spec (POST /auth/otp/request → /auth/otp/verify → JWT) — Tasks 7, 9, 10, 14 ✓
- JWT payload `{user_id, organization_id, role, exp}` — Task 2 ✓
- Auth middleware extracts JWT, populates ctx — Task 11 ✓
- Role middleware — Task 12 ✓
- Spec said "no passwords (OTP-only)" — Task 9 enforces this (no password field used) ✓
- Refresh token cookie — explicitly deferred. Spec D8 layered defenses still hold (Auth populates ctx; Role/Tenant/RLSContext already wired by earlier plans).

**Placeholder scan:** No "TBD"/"TODO"/"add error handling". Every step is concrete. Smoke test in Task 15 has one explicit "<the-6-digit-code>" placeholder, but that's a runtime value the operator pastes, not a plan defect.

**Type / signature consistency:**
- `auth.Claims{UserID, OrgID, Role, TTL}` consistent across Tasks 2, 9, 11.
- `auth.Identity{UserID, OrgID, Role}` consistent across Tasks 4, 11, 12.
- `auth.NewIssuer/NewVerifier(secret []byte, issuer string)` matches in Tasks 2, 14.
- `service.NewAuthService(otps, users, emailer, iss, ttl)` matches Task 9 + Task 14 wiring.
- `apihandler.NewAuthHandler(svc AuthService)` matches Task 10 + Task 14 wiring.
- `middleware.Auth(*auth.Verifier)` and `middleware.Role(allowed ...string)` consistent in tests + main.go.
- Email package: `Sender.Send(ctx, Message)` and `Message{To, Subject, Body}` consistent.
- `repo.OtpRepo`: `Create`, `ConsumeActive(email)` consistent.

**Scope:** 16 tasks. Substantive: 2, 3, 5, 7, 9, 10, 11, 14. Trivial: 1, 4, 6, 8, 12, 13. Verification: 15, 16.
