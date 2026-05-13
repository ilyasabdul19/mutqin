// api/internal/handler/landing/handler_test.go
package landing_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/handler/landing"
	"github.com/ilyas/mutqin-api/internal/model"
	"github.com/ilyas/mutqin-api/internal/repo"
)

type stubOrgLookup struct {
	stored map[string]*model.Organization
}

func (s *stubOrgLookup) GetBySlugAdmin(_ context.Context, slug string) (*model.Organization, error) {
	if o, ok := s.stored[slug]; ok {
		return o, nil
	}
	return nil, repo.ErrNotFound
}

type stubAnnouncements struct {
	gotOrg   uuid.UUID
	gotLimit int
	returned []model.Announcement
	err      error
}

func (s *stubAnnouncements) ListByOrgPublic(_ context.Context, orgID uuid.UUID, limit int) ([]model.Announcement, error) {
	s.gotOrg = orgID
	s.gotLimit = limit
	return s.returned, s.err
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestLanding_Center_RendersAnnouncements(t *testing.T) {
	orgID := uuid.New()
	orgs := &stubOrgLookup{stored: map[string]*model.Organization{
		"al-falah": {ID: orgID, Slug: "al-falah", Name: "Al Falah"},
	}}
	annStub := &stubAnnouncements{returned: []model.Announcement{
		{ID: uuid.New(), OrganizationID: orgID, Title: "Eid Break", Body: "Classes resume Monday.", CreatedAt: time.Now()},
		{ID: uuid.New(), OrganizationID: orgID, Title: "Trip", Body: "Beach trip Friday.", CreatedAt: time.Now()},
	}}

	h, err := landing.New(orgs, annStub, nil, quietLogger())
	if err != nil {
		t.Fatalf("landing.New: %v", err)
	}

	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/al-falah")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	if !strings.Contains(body, "Al Falah") {
		t.Fatalf("missing org name in body")
	}
	if !strings.Contains(body, "Eid Break") || !strings.Contains(body, "Trip") {
		t.Fatalf("missing announcement titles, body=%s", body)
	}
	if !strings.Contains(body, "Classes resume Monday.") {
		t.Fatalf("missing announcement body, page=%s", body)
	}
	if annStub.gotOrg != orgID {
		t.Fatalf("ListByOrgPublic called with %s want %s", annStub.gotOrg, orgID)
	}
	if annStub.gotLimit <= 0 {
		t.Fatalf("limit not forwarded: %d", annStub.gotLimit)
	}
}

func TestLanding_Center_NoAnnouncementsHidesSection(t *testing.T) {
	orgID := uuid.New()
	orgs := &stubOrgLookup{stored: map[string]*model.Organization{
		"empty": {ID: orgID, Slug: "empty", Name: "Empty Center"},
	}}
	annStub := &stubAnnouncements{returned: nil}

	h, err := landing.New(orgs, annStub, nil, quietLogger())
	if err != nil {
		t.Fatalf("landing.New: %v", err)
	}

	srv := httptest.NewServer(h.Routes())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/empty")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)
	if strings.Contains(body, "announcements") || strings.Contains(body, "Announcements") {
		// "Announcements" only appears inside the section's h2; if the
		// section is hidden when empty there should be no occurrence.
		t.Fatalf("announcements section rendered with no items, body=%s", body)
	}
}
