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
	GetByID(ctx context.Context, id uuid.UUID) (*model.Organization, error)
	GetBySlug(ctx context.Context, slug string) (*model.Organization, error)
	List(ctx context.Context, limit, offset int) ([]model.Organization, error)
	Update(ctx context.Context, o *model.Organization) error
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

// UpdateLandingInput is the validated payload for the landing-page editor.
// Every field is optional — only non-nil pointers / non-zero slices are
// applied. nil/empty leaves the existing value untouched.
type UpdateLandingInput struct {
	Description *string
	LogoURL     *string
	City        *string
	Schedule    []byte
}

// UpdateLanding mutates the landing-page-visible fields on an organization.
// Authorization (e.g. center_admin owns this org) is enforced upstream by
// the handler via the JWT-derived OrgID, so this entry-point trusts orgID.
// Returns ErrInvalidInput if the org cannot be loaded.
func (s *OrganizationService) UpdateLanding(ctx context.Context, actorID, orgID uuid.UUID, in UpdateLandingInput) error {
	org, err := s.orgs.GetByID(ctx, orgID)
	if err != nil {
		return fmt.Errorf("lookup org: %w", err)
	}
	if in.Description != nil {
		org.Description = in.Description
	}
	if in.LogoURL != nil {
		org.LogoURL = in.LogoURL
	}
	if in.City != nil {
		org.City = in.City
	}
	if len(in.Schedule) > 0 {
		org.Schedule = in.Schedule
	}
	if err := s.orgs.Update(ctx, org); err != nil {
		return fmt.Errorf("update org: %w", err)
	}

	tt := "organization"
	tid := org.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "update_landing",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}
