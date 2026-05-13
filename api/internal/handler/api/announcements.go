// api/internal/handler/api/announcements.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// AnnouncementService is the surface the handler needs. Production
// implementation is *service.AnnouncementService; tests provide a stub.
type AnnouncementService interface {
	Create(ctx context.Context, actor, orgID uuid.UUID, in service.CreateAnnouncementInput) (*model.Announcement, error)
	ListByOrg(ctx context.Context, limit, offset int) ([]model.Announcement, error)
	Delete(ctx context.Context, actor, id uuid.UUID) error
}

type AnnouncementsHandler struct {
	svc AnnouncementService
}

func NewAnnouncementsHandler(svc AnnouncementService) *AnnouncementsHandler {
	return &AnnouncementsHandler{svc: svc}
}

type createAnnouncementBody struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (h *AnnouncementsHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	var b createAnnouncementBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	in := service.CreateAnnouncementInput{Title: b.Title, Body: b.Body}
	a, err := h.svc.Create(r.Context(), id.UserID, *id.OrgID, in)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "title and body are required")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not create announcement")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": a})
}

func (h *AnnouncementsHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.svc.ListByOrg(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

func (h *AnnouncementsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	announcementID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid announcement id")
		return
	}
	if err := h.svc.Delete(r.Context(), id.UserID, announcementID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "announcement not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not delete announcement")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
