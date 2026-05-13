// api/internal/handler/api/recitations.go
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

// RecitationService is the surface RecitationsHandler needs. Production
// implementation is *service.RecitationService; tests provide a stub.
type RecitationService interface {
	Record(ctx context.Context, actor, orgID uuid.UUID, in service.RecordInput) (*model.Recitation, error)
	RecordBatch(ctx context.Context, actor, orgID uuid.UUID, ins []service.RecordInput) ([]model.Recitation, error)
	ListForStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error)
	GetLatest(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error)
}

type RecitationsHandler struct {
	svc RecitationService
}

func NewRecitationsHandler(svc RecitationService) *RecitationsHandler {
	return &RecitationsHandler{svc: svc}
}

type recitationBody struct {
	StudentID   string  `json:"student_id"`
	HalaqahID   string  `json:"halaqah_id"`
	Type        string  `json:"type"`
	SurahNumber int     `json:"surah_number"`
	AyahFrom    int     `json:"ayah_from"`
	AyahTo      int     `json:"ayah_to"`
	Grade       string  `json:"grade"`
	Notes       *string `json:"notes"`
	ClientID    *string `json:"client_id"`
}

func (b recitationBody) toInput() (service.RecordInput, error) {
	sid, err := uuid.Parse(strings.TrimSpace(b.StudentID))
	if err != nil {
		return service.RecordInput{}, errors.New("invalid student_id")
	}
	hid, err := uuid.Parse(strings.TrimSpace(b.HalaqahID))
	if err != nil {
		return service.RecordInput{}, errors.New("invalid halaqah_id")
	}
	return service.RecordInput{
		StudentID:   sid,
		HalaqahID:   hid,
		Type:        b.Type,
		SurahNumber: b.SurahNumber,
		AyahFrom:    b.AyahFrom,
		AyahTo:      b.AyahTo,
		Grade:       b.Grade,
		Notes:       b.Notes,
		ClientID:    b.ClientID,
	}, nil
}

func (h *RecitationsHandler) Record(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "org context required")
		return
	}
	var b recitationBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	in, err := b.toInput()
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
		return
	}
	rec, err := h.svc.Record(r.Context(), id.UserID, *id.OrgID, in)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRecitationInput) {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not record recitation")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": rec})
}

func (h *RecitationsHandler) RecordBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "org context required")
		return
	}
	var bs []recitationBody
	if err := json.NewDecoder(r.Body).Decode(&bs); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	ins := make([]service.RecordInput, 0, len(bs))
	for i, b := range bs {
		in, err := b.toInput()
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "input "+strconv.Itoa(i)+": "+err.Error())
			return
		}
		ins = append(ins, in)
	}
	recs, err := h.svc.RecordBatch(r.Context(), id.UserID, *id.OrgID, ins)
	if err != nil {
		if errors.Is(err, service.ErrInvalidRecitationInput) {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not record recitations")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": recs,
		"meta": map[string]int{"count": len(recs)},
	})
}

func (h *RecitationsHandler) ListByStudent(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	studentID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid student id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	out, err := h.svc.ListForStudent(r.Context(), studentID, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, out, len(out))
}

func (h *RecitationsHandler) LatestByStudent(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	rawID := chi.URLParam(r, "id")
	studentID, err := uuid.Parse(rawID)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid student id")
		return
	}
	rec, err := h.svc.GetLatest(r.Context(), studentID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "no recitation found for student")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "lookup failed")
		return
	}
	response.Success(w, rec)
}
