// api/internal/handler/api/attendance.go
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
	"github.com/ilyas/mutqin-api/internal/response"
	"github.com/ilyas/mutqin-api/internal/service"
)

// AttendanceService is the surface AttendanceHandler depends on. Production
// implementation is *service.AttendanceService; tests provide a stub.
type AttendanceService interface {
	MarkBatch(ctx context.Context, actor, orgID, halaqahID uuid.UUID, date time.Time, marks []service.MarkInput) ([]model.Attendance, error)
	ListByHalaqahDate(ctx context.Context, halaqahID uuid.UUID, date time.Time) ([]model.Attendance, error)
	ListByStudent(ctx context.Context, studentID uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
	ListByOrg(ctx context.Context, halaqahID *uuid.UUID, from, to time.Time, limit, offset int) ([]model.Attendance, error)
}

type AttendanceHandler struct {
	svc AttendanceService
}

func NewAttendanceHandler(svc AttendanceService) *AttendanceHandler {
	return &AttendanceHandler{svc: svc}
}

type markEntry struct {
	StudentID string  `json:"student_id"`
	Status    string  `json:"status"`
	ClientID  *string `json:"client_id"`
}

type markBatchBody struct {
	Date  string      `json:"date"`
	Marks []markEntry `json:"marks"`
}

func (h *AttendanceHandler) MarkBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.From(r.Context())
	if !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	if id.OrgID == nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "org context required")
		return
	}
	halaqahID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah id")
		return
	}

	var b markBatchBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid body")
		return
	}
	date, err := time.Parse("2006-01-02", strings.TrimSpace(b.Date))
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "date must be YYYY-MM-DD")
		return
	}

	marks := make([]service.MarkInput, 0, len(b.Marks))
	for i, m := range b.Marks {
		stuID, err := uuid.Parse(strings.TrimSpace(m.StudentID))
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid student_id at index "+strconv.Itoa(i))
			return
		}
		marks = append(marks, service.MarkInput{
			StudentID: stuID,
			Status:    strings.TrimSpace(m.Status),
			ClientID:  m.ClientID,
		})
	}

	rows, err := h.svc.MarkBatch(r.Context(), id.UserID, *id.OrgID, halaqahID, date, marks)
	if err != nil {
		if errors.Is(err, service.ErrInvalidAttendance) {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "could not mark attendance")
		return
	}
	response.SuccessList(w, rows, len(rows))
}

func (h *AttendanceHandler) ListByHalaqahDate(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	halaqahID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah id")
		return
	}
	rawDate := strings.TrimSpace(r.URL.Query().Get("date"))
	if rawDate == "" {
		rawDate = time.Now().UTC().Format("2006-01-02")
	}
	date, err := time.Parse("2006-01-02", rawDate)
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "date must be YYYY-MM-DD")
		return
	}
	rows, err := h.svc.ListByHalaqahDate(r.Context(), halaqahID, date)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, rows, len(rows))
}

func (h *AttendanceHandler) ListByStudent(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	stuID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid student id")
		return
	}

	from, to, err := parseDateRange(r, "from", "to")
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	rows, err := h.svc.ListByStudent(r.Context(), stuID, from, to, limit, offset)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			response.Error(w, http.StatusNotFound, response.CodeNotFound, "not found")
			return
		}
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, rows, len(rows))
}

func (h *AttendanceHandler) ListByOrg(w http.ResponseWriter, r *http.Request) {
	if _, ok := auth.From(r.Context()); !ok {
		response.Error(w, http.StatusUnauthorized, response.CodeUnauthorized, "auth required")
		return
	}
	var halaqahID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("halaqah_id")); raw != "" {
		hid, err := uuid.Parse(raw)
		if err != nil {
			response.Error(w, http.StatusBadRequest, response.CodeValidationError, "invalid halaqah_id")
			return
		}
		halaqahID = &hid
	}
	from, to, err := parseDateRange(r, "date_from", "date_to")
	if err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeValidationError, err.Error())
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	rows, err := h.svc.ListByOrg(r.Context(), halaqahID, from, to, limit, offset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, response.CodeInternalError, "list failed")
		return
	}
	response.SuccessList(w, rows, len(rows))
}

// parseDateRange reads `from`/`to` query params named by keyFrom/keyTo. If
// absent, defaults to (today-30days, today). Returns 400-mappable error on
// malformed values.
func parseDateRange(r *http.Request, keyFrom, keyTo string) (time.Time, time.Time, error) {
	today := time.Now().UTC()
	defaultFrom := today.AddDate(0, 0, -30)

	var (
		from = defaultFrom
		to   = today
		err  error
	)
	if raw := strings.TrimSpace(r.URL.Query().Get(keyFrom)); raw != "" {
		from, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New(keyFrom + " must be YYYY-MM-DD")
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get(keyTo)); raw != "" {
		to, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New(keyTo + " must be YYYY-MM-DD")
		}
	}
	return from, to, nil
}
