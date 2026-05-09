// api/internal/handler/api/teachers_test.go
package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/auth"
	apihandler "github.com/ilyas/mutqin-api/internal/handler/api"
	"github.com/ilyas/mutqin-api/internal/model"
)

type stubTeacherInviteIssuer struct {
	gotActor    uuid.UUID
	gotOrgID    uuid.UUID
	generated   *model.Invite
	generateErr error
}

func (s *stubTeacherInviteIssuer) GenerateForTeacher(_ context.Context, actor, orgID uuid.UUID) (*model.Invite, error) {
	s.gotActor = actor
	s.gotOrgID = orgID
	if s.generateErr != nil {
		return nil, s.generateErr
	}
	if s.generated != nil {
		return s.generated, nil
	}
	return &model.Invite{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Role:           "teacher",
		Token:          "teacher-fake-token",
		ExpiresAt:      time.Now().Add(72 * time.Hour),
	}, nil
}

func TestTeachers_GenerateInvite_RequiresIdentity(t *testing.T) {
	h := apihandler.NewTeachersHandler(&stubTeacherInviteIssuer{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teachers/invite", nil)
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}
}

func TestTeachers_GenerateInvite_RequiresOrgID(t *testing.T) {
	h := apihandler.NewTeachersHandler(&stubTeacherInviteIssuer{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/teachers/invite", nil)
	// super_admin without OrgID
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), Role: "super_admin"}))
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", rec.Code)
	}
}

func TestTeachers_GenerateInvite_Success(t *testing.T) {
	issuer := &stubTeacherInviteIssuer{}
	h := apihandler.NewTeachersHandler(issuer)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/teachers/invite", nil)
	actor := uuid.New()
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: actor, OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if issuer.gotActor != actor {
		t.Fatal("actor mismatch")
	}
	if issuer.gotOrgID != orgID {
		t.Fatal("org_id mismatch")
	}

	var resp struct {
		Data struct {
			Token     string    `json:"token"`
			ExpiresAt time.Time `json:"expires_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if resp.Data.Token == "" {
		t.Fatal("token empty in response")
	}
	if resp.Data.ExpiresAt.IsZero() {
		t.Fatal("expires_at empty")
	}
}

func TestTeachers_GenerateInvite_ServiceError(t *testing.T) {
	issuer := &stubTeacherInviteIssuer{generateErr: errors.New("boom")}
	h := apihandler.NewTeachersHandler(issuer)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/teachers/invite", nil)
	orgID := uuid.New()
	req = req.WithContext(auth.With(req.Context(), auth.Identity{UserID: uuid.New(), OrgID: &orgID, Role: "center_admin"}))
	rec := httptest.NewRecorder()
	h.GenerateInvite(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 got %d", rec.Code)
	}
}
