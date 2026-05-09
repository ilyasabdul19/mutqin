// api/internal/handler/api/halaqat.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// HalaqatService is the surface HalaqatHandler needs. Production implementation
// is *service.HalaqahService; tests provide a stub.
type HalaqatService interface {
	Create(ctx context.Context, actor, orgID uuid.UUID, in service.CreateHalaqahInput) (*model.Halaqah, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error)
	List(ctx context.Context, limit, offset int) ([]model.Halaqah, error)
	Update(ctx context.Context, actor uuid.UUID, h *model.Halaqah) error
}

type HalaqatHandler struct {
	svc HalaqatService
}

func NewHalaqatHandler(svc HalaqatService) *HalaqatHandler {
	return &HalaqatHandler{svc: svc}
}

type createHalaqahBody struct {
	Name        string  `json:"name"`
	MaxCapacity int     `json:"max_capacity"`
	TeacherID   *string `json:"teacher_id"`
}

func (h *HalaqatHandler) Create(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	var b createHalaqahBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "name is required")
		return
	}
	if b.MaxCapacity <= 0 {
		b.MaxCapacity = 30
	}
	in := service.CreateHalaqahInput{
		Name:        b.Name,
		MaxCapacity: b.MaxCapacity,
	}
	if b.TeacherID != nil && strings.TrimSpace(*b.TeacherID) != "" {
		tid, err := uuid.Parse(*b.TeacherID)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid teacher_id")
			return
		}
		in.TeacherID = &tid
	}

	hl, err := h.svc.Create(r.Context(), id.UserID, *id.OrgID, in)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not create halaqah")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": hl})
}

func (h *HalaqatHandler) List(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.svc.List(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

func (h *HalaqatHandler) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid id")
		return
	}
	hl, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "halaqah not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "lookup failed")
		return
	}
	response.Success(w, hl)
}

type updateHalaqahBody struct {
	Name        *string `json:"name"`
	MaxCapacity *int    `json:"max_capacity"`
	TeacherID   *string `json:"teacher_id"`
	Status      *string `json:"status"`
}

func (h *HalaqatHandler) Update(w http.ResponseWriter, r *http.Request) {
	idn, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid id")
		return
	}
	existing, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "halaqah not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "lookup failed")
		return
	}

	var b updateHalaqahBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	if b.Name != nil {
		trimmed := strings.TrimSpace(*b.Name)
		if trimmed == "" {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "name cannot be empty")
			return
		}
		existing.Name = trimmed
	}
	if b.MaxCapacity != nil {
		if *b.MaxCapacity > 0 {
			existing.MaxCapacity = *b.MaxCapacity
		}
	}
	if b.TeacherID != nil {
		raw := strings.TrimSpace(*b.TeacherID)
		if raw == "" {
			existing.TeacherID = nil
		} else {
			tid, err := uuid.Parse(raw)
			if err != nil {
				response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid teacher_id")
				return
			}
			existing.TeacherID = &tid
		}
	}
	if b.Status != nil {
		existing.Status = strings.TrimSpace(*b.Status)
	}

	if err := h.svc.Update(r.Context(), idn.UserID, existing); err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not update halaqah")
		return
	}
	response.Success(w, existing)
}
