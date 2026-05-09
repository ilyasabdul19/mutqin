// api/internal/service/organization.go
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

type orgRepo interface {
	Create(ctx context.Context, o *model.Organization) error
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	List(ctx context.Context, limit, offset int) ([]model.Organization, error)
}

type auditRepo interface {
	Create(ctx context.Context, row *model.AuditLog) error
}

type OrganizationService struct {
	orgs  orgRepo
	audit auditRepo
}

func NewOrganizationService(orgs orgRepo, audit auditRepo) *OrganizationService {
	return &OrganizationService{orgs: orgs, audit: audit}
}

// CreateOrgInput is the validated payload for creating a new organization.
type CreateOrgInput struct {
	Name        string
	Slug        string
	Country     string
	City        *string
	Description *string
	Tier        string // empty → "free"
}

func (s *OrganizationService) Create(ctx context.Context, actorID uuid.UUID, in CreateOrgInput) (*model.Organization, error) {
	o := &model.Organization{
		Name:        in.Name,
		Slug:        in.Slug,
		Country:     in.Country,
		City:        in.City,
		Description: in.Description,
		Tier:        in.Tier,
		Status:      "active",
	}
	if o.Tier == "" {
		o.Tier = "free"
	}
	if err := s.orgs.Create(ctx, o); err != nil {
		return nil, fmt.Errorf("create org: %w", err)
	}

	details, _ := json.Marshal(map[string]string{"slug": o.Slug, "name": o.Name})
	tt := "organization"
	tid := o.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "create_organization",
		TargetType: &tt,
		TargetID:   &tid,
		Details:    details,
	}) // audit failure does not roll back the org create
	return o, nil
}

func (s *OrganizationService) GetBySlug(ctx context.Context, slug string) (*model.Organization, error) {
	return s.orgs.GetBySlug(ctx, slug)
}

func (s *OrganizationService) List(ctx context.Context, limit, offset int) ([]model.Organization, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.orgs.List(ctx, limit, offset)
}
