// api/internal/handler/api/platform_stats.go
package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
)

// PlatformStatsService is the surface PlatformStatsHandler needs. Production
// implementation is *service.PlatformService; tests provide a stub.
type PlatformStatsService interface {
	Stats(ctx context.Context) (*repo.PlatformStats, error)
	AuditLog(ctx context.Context, actorID uuid.UUID, limit, offset int) ([]model.AuditLog, error)
}

type PlatformStatsHandler struct {
	svc PlatformStatsService
}

func NewPlatformStatsHandler(svc PlatformStatsService) *PlatformStatsHandler {
	return &PlatformStatsHandler{svc: svc}
}

// Stats returns the platform-wide counter rollup.
func (h *PlatformStatsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	s, err := h.svc.Stats(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not compute platform stats")
		return
	}
	response.Success(w, s)
}

// AuditLog returns audit_log rows filtered by actor_id. Missing actor_id
// returns an empty list (200) so the UI can render a "select a user" state
// without a special-case error path. Bad UUIDs map to 400.
func (h *PlatformStatsHandler) AuditLog(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawActor := strings.TrimSpace(r.URL.Query().Get("actor_id"))
	if rawActor == "" {
		// No actor filter — return empty list rather than dumping the whole
		// audit table or 4xx-ing the request.
		response.SuccessList(w, []model.AuditLog{}, 0)
		return
	}
	actorID, err := uuid.Parse(rawActor)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid actor_id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	rows, err := h.svc.AuditLog(r.Context(), actorID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "audit lookup failed")
		return
	}
	response.SuccessList(w, rows, len(rows))
}
