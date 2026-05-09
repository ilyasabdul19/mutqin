// api/internal/handler/api/invite.go
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

type InviteAcceptor interface {
	AcceptInvite(ctx context.Context, token, email, name string) error
}

type InviteAcceptHandler struct {
	svc InviteAcceptor
}

func NewInviteAcceptHandler(s InviteAcceptor) *InviteAcceptHandler {
	return &InviteAcceptHandler{svc: s}
}

type inviteAcceptBody struct {
	Token string `json:"token"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *InviteAcceptHandler) Accept(w http.ResponseWriter, r *http.Request) {
	var b inviteAcceptBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	b.Token = strings.TrimSpace(b.Token)
	b.Email = strings.TrimSpace(strings.ToLower(b.Email))
	b.Name = strings.TrimSpace(b.Name)
	if b.Token == "" || b.Email == "" || b.Name == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "token, email, name required")
		return
	}
	err := h.svc.AcceptInvite(r.Context(), b.Token, b.Email, b.Name)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInvite) {
			response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "invalid invite")
			return
		}
		if errors.Is(err, service.ErrEmailTaken) {
			response.Error(w, http.StatusConflict, response.CodeConflict, "email already in use")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not accept invite")
		return
	}
	response.Success(w, map[string]string{"status": "code_sent"})
}
