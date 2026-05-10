// api/internal/handler/api/teachers.go
package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/response"
)

// TeacherInviteIssuer is the surface TeachersHandler needs. Production
// implementation is *service.InviteService; tests provide a stub.
type TeacherInviteIssuer interface {
	GenerateForTeacher(ctx context.Context, actor, orgID uuid.UUID) (*model.Invite, error)
}

type TeachersHandler struct {
	invites TeacherInviteIssuer
}

func NewTeachersHandler(invites TeacherInviteIssuer) *TeachersHandler {
	return &TeachersHandler{invites: invites}
}

// GenerateInvite issues a teacher invite. The orgID comes from the caller's
// authenticated identity (center_admin); super_admin without an OrgID is
// rejected because there's no implicit tenant to attach the invite to.
func (h *TeachersHandler) GenerateInvite(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	inv, err := h.invites.GenerateForTeacher(r.Context(), id.UserID, *id.OrgID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not generate invite")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		"token":      inv.Token,
		"expires_at": inv.ExpiresAt,
	}})
}
