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
