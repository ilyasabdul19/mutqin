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
