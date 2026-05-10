// api/internal/service/halaqah.go
package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

// halaqahRepoIface is the package-private repo interface the HalaqahService
// depends on. Methods take a ctx-with-tenant — the caller (middleware) is
// responsible for setting tenant; this service never sets it itself.
type halaqahRepoIface interface {
	Create(ctx context.Context, h *model.Halaqah) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error)
	ListByOrg(ctx context.Context, limit, offset int) ([]model.Halaqah, error)
	Update(ctx context.Context, h *model.Halaqah) error
}

type HalaqahService struct {
	halaqat halaqahRepoIface
	audit   auditRepo
}

func NewHalaqahService(halaqat halaqahRepoIface, audit auditRepo) *HalaqahService {
	return &HalaqahService{halaqat: halaqat, audit: audit}
}

// CreateHalaqahInput is the validated payload for creating a new halaqah.
type CreateHalaqahInput struct {
	Name        string
	MaxCapacity int // <= 0 → defaults to 30
	TeacherID   *uuid.UUID
	Schedule    []byte
}

func (s *HalaqahService) Create(ctx context.Context, actorID, orgID uuid.UUID, in CreateHalaqahInput) (*model.Halaqah, error) {
	h := &model.Halaqah{
		OrganizationID: orgID,
		Name:           in.Name,
		MaxCapacity:    in.MaxCapacity,
		TeacherID:      in.TeacherID,
		Schedule:       in.Schedule,
		Status:         "active",
	}
	if h.MaxCapacity <= 0 {
		h.MaxCapacity = 30
	}
	if err := s.halaqat.Create(ctx, h); err != nil {
		return nil, fmt.Errorf("create halaqah: %w", err)
	}

	tt := "halaqah"
	tid := h.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "create_halaqah",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return h, nil
}

func (s *HalaqahService) GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error) {
	return s.halaqat.GetByID(ctx, id)
}

func (s *HalaqahService) List(ctx context.Context, limit, offset int) ([]model.Halaqah, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.halaqat.ListByOrg(ctx, limit, offset)
}

func (s *HalaqahService) Update(ctx context.Context, actorID uuid.UUID, h *model.Halaqah) error {
	if err := s.halaqat.Update(ctx, h); err != nil {
		return fmt.Errorf("update halaqah: %w", err)
	}
	tt := "halaqah"
	tid := h.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "update_halaqah",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}

func (s *HalaqahService) Deactivate(ctx context.Context, actorID, id uuid.UUID) error {
	h, err := s.halaqat.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("lookup halaqah: %w", err)
	}
	h.Status = "inactive"
	if err := s.halaqat.Update(ctx, h); err != nil {
		return fmt.Errorf("update halaqah: %w", err)
	}
	tt := "halaqah"
	tid := h.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "deactivate_halaqah",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}
