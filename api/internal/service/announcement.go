// api/internal/service/announcement.go
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

// ErrInvalidInput is returned when a service-level validation rejects the
// input (e.g. blank title/body). Handler layer maps this to 400.
var ErrInvalidInput = errors.New("service: invalid input")

// announcementRepoIface is the package-private repo surface used by
// AnnouncementService. Methods take a ctx-with-tenant — the caller
// (middleware) is responsible for setting tenant.
type announcementRepoIface interface {
	Create(ctx context.Context, a *model.Announcement) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Announcement, error)
	ListByOrg(ctx context.Context, limit, offset int) ([]model.Announcement, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type AnnouncementService struct {
	announcements announcementRepoIface
	audit         auditRepo
}

func NewAnnouncementService(announcements announcementRepoIface, audit auditRepo) *AnnouncementService {
	return &AnnouncementService{announcements: announcements, audit: audit}
}

// CreateAnnouncementInput is the validated payload for creating an
// announcement. Title + Body are required.
type CreateAnnouncementInput struct {
	Title string
	Body  string
}

func (s *AnnouncementService) Create(ctx context.Context, actorID, orgID uuid.UUID, in CreateAnnouncementInput) (*model.Announcement, error) {
	title := strings.TrimSpace(in.Title)
	body := strings.TrimSpace(in.Body)
	if title == "" || body == "" {
		return nil, ErrInvalidInput
	}

	a := &model.Announcement{
		OrganizationID: orgID,
		Title:          title,
		Body:           body,
	}
	if err := s.announcements.Create(ctx, a); err != nil {
		return nil, fmt.Errorf("create announcement: %w", err)
	}

	tt := "announcement"
	tid := a.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "create_announcement",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return a, nil
}

func (s *AnnouncementService) Delete(ctx context.Context, actorID, id uuid.UUID) error {
	if err := s.announcements.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete announcement: %w", err)
	}
	tt := "announcement"
	tid := id
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "delete_announcement",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}

func (s *AnnouncementService) ListByOrg(ctx context.Context, limit, offset int) ([]model.Announcement, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.announcements.ListByOrg(ctx, limit, offset)
}
