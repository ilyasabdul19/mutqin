// api/internal/handler/api/students.go
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
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// StudentService is the surface StudentsHandler needs. Production implementation
// is *service.StudentService; tests provide a stub.
type StudentService interface {
	Enroll(ctx context.Context, actor, orgID, halaqahID uuid.UUID, in service.EnrollStudentInput) (*model.Student, error)
	ListByHalaqah(ctx context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error)
	Transfer(ctx context.Context, actor, studentID, newHalaqahID, orgID uuid.UUID) error
}

type StudentsHandler struct {
	svc StudentService
}

func NewStudentsHandler(svc StudentService) *StudentsHandler {
	return &StudentsHandler{svc: svc}
}

type enrollStudentBody struct {
	Name        string  `json:"name"`
	Age         *int    `json:"age"`
	ParentPhone *string `json:"parent_phone"`
	ParentEmail *string `json:"parent_email"`
	HifzLevel   *string `json:"hifz_level"`
}

func (h *StudentsHandler) Enroll(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	rawID := chi.URLParam(r, "id")
	halaqahID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah id")
		return
	}
	var b enrollStudentBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "name is required")
		return
	}
	in := service.EnrollStudentInput{
		Name:        b.Name,
		Age:         b.Age,
		ParentPhone: b.ParentPhone,
		ParentEmail: b.ParentEmail,
		HifzLevel:   b.HifzLevel,
	}
	st, err := h.svc.Enroll(r.Context(), id.UserID, *id.OrgID, halaqahID, in)
	if err != nil {
		if errors.Is(err, service.ErrCrossTenant) {
			response.Error(w, http.StatusForbidden, response.CodeForbidden, "halaqah belongs to a different organization")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not enroll student")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": st})
}

func (h *StudentsHandler) ListByHalaqah(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	halaqahID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.svc.ListByHalaqah(r.Context(), halaqahID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

type transferStudentBody struct {
	NewHalaqahID string `json:"new_halaqah_id"`
}

func (h *StudentsHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	rawID := chi.URLParam(r, "id")
	studentID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid student id")
		return
	}
	var b transferStudentBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	b.NewHalaqahID = strings.TrimSpace(b.NewHalaqahID)
	newHalaqahID, err := uuid.Parse(b.NewHalaqahID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid new_halaqah_id")
		return
	}
	if err := h.svc.Transfer(r.Context(), id.UserID, studentID, newHalaqahID, *id.OrgID); err != nil {
		if errors.Is(err, service.ErrCrossTenant) {
			response.Error(w, http.StatusForbidden, response.CodeForbidden, "target halaqah belongs to a different organization")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not transfer student")
		return
	}
	response.Success(w, map[string]string{"status": "transferred"})
}
