// api/internal/handler/api/dashboard.go
package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
)

// DashboardService is the surface DashboardHandler needs. Production
// implementation is *service.DashboardService; tests provide a stub.
type DashboardService interface {
	CenterStats(ctx context.Context) (*repo.CenterStats, error)
	AttendanceTrends(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time) ([]repo.AttendanceTrendRow, error)
	RecitationActivity(ctx context.Context, from, to time.Time) ([]repo.RecitationActivityRow, error)
}

type DashboardHandler struct {
	svc DashboardService
}

func NewDashboardHandler(svc DashboardService) *DashboardHandler {
	return &DashboardHandler{svc: svc}
}

// Stats returns the center-scoped counter rollup.
func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	stats, err := h.svc.CenterStats(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not compute stats")
		return
	}
	response.Success(w, stats)
}

// AttendanceTrends returns one row per date in the (optional) range. Defaults
// to today-30d .. today when from/to are omitted; bad dates yield 400.
func (h *DashboardHandler) AttendanceTrends(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	from, to, err := parseDateRange(r, "from", "to")
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
		return
	}
	var halaqahID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("halaqah_id")); raw != "" {
		id, perr := uuid.Parse(raw)
		if perr != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah_id")
			return
		}
		halaqahID = &id
	}
	rows, err := h.svc.AttendanceTrends(r.Context(), halaqahID, from, to)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not compute trends")
		return
	}
	response.SuccessList(w, rows, len(rows))
}

// RecitationActivity returns per-halaqah counts in the (optional) range.
func (h *DashboardHandler) RecitationActivity(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	from, to, err := parseDateRange(r, "from", "to")
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
		return
	}
	rows, err := h.svc.RecitationActivity(r.Context(), from, to)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not compute activity")
		return
	}
	response.SuccessList(w, rows, len(rows))
}
