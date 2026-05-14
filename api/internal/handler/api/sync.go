// api/internal/handler/api/sync.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// SyncService is the surface SyncHandler needs. Production implementation is
// *service.SyncService; tests provide a stub.
type SyncService interface {
	Push(ctx context.Context, actor, orgID uuid.UUID, in service.PushInput) (*service.PushResult, error)
	Pull(ctx context.Context, since time.Time, limit int) (*service.PullResult, error)
}

type SyncHandler struct {
	svc SyncService
}

func NewSyncHandler(svc SyncService) *SyncHandler {
	return &SyncHandler{svc: svc}
}

type syncPushBody struct {
	Recitations []model.Recitation `json:"recitations"`
	Attendance  []model.Attendance `json:"attendance"`
}

// Push accepts a batch of offline-collected recitations + attendance rows
// and forwards them to the SyncService. Server stamps org + teacher; client
// fields for those are ignored.
func (h *SyncHandler) Push(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	var b syncPushBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	res, err := h.svc.Push(r.Context(), id.UserID, *id.OrgID, service.PushInput{
		Recitations: b.Recitations,
		Attendance:  b.Attendance,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidSyncInput) {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not push sync batch")
		return
	}
	response.Success(w, res)
}

// Pull returns rows for the tenant on ctx where recorded_at > since.
// `since` is a RFC3339 timestamp; absent → epoch (full backfill).
func (h *SyncHandler) Pull(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}

	rawSince := r.URL.Query().Get("since")
	var since time.Time
	if rawSince == "" {
		since = time.Unix(0, 0)
	} else {
		t, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid since (want RFC3339)")
			return
		}
		since = t
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	res, err := h.svc.Pull(r.Context(), since, limit)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not pull sync changes")
		return
	}
	response.Success(w, res)
}
