// api/internal/service/student.go
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

// ErrCrossTenant is returned when a halaqah lookup yields a halaqah owned
// by a different organization than the caller's claimed org.
var ErrCrossTenant = errors.New("service: halaqah belongs to a different organization")

// studentRepoIface is the package-private student repo interface.
type studentRepoIface interface {
	Create(ctx context.Context, s *model.Student) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Student, error)
	ListByHalaqah(ctx context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error)
	ListByOrg(ctx context.Context, limit, offset int) ([]model.Student, error)
	Update(ctx context.Context, s *model.Student) error
	Transfer(ctx context.Context, studentID, newHalaqahID uuid.UUID) error
}

// halaqahReader is a read-only halaqah lookup used for cross-tenant validation
// during student enrollment and transfer. Kept narrow on purpose so test mocks
// don't have to satisfy the full repo interface.
type halaqahReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Halaqah, error)
}

type StudentService struct {
	students studentRepoIface
	halaqat  halaqahReader
	audit    auditRepo
}

func NewStudentService(students studentRepoIface, halaqat halaqahReader, audit auditRepo) *StudentService {
	return &StudentService{students: students, halaqat: halaqat, audit: audit}
}

// EnrollStudentInput is the validated payload for enrolling a new student.
type EnrollStudentInput struct {
	Name        string
	Age         *int
	ParentPhone *string
	ParentEmail *string
	HifzLevel   *string
}

func (s *StudentService) Enroll(ctx context.Context, actorID, orgID, halaqahID uuid.UUID, in EnrollStudentInput) (*model.Student, error) {
	h, err := s.halaqat.GetByID(ctx, halaqahID)
	if err != nil {
		return nil, fmt.Errorf("lookup halaqah: %w", err)
	}
	if h.OrganizationID != orgID {
		return nil, ErrCrossTenant
	}

	st := &model.Student{
		OrganizationID: orgID,
		HalaqahID:      &halaqahID,
		Name:           in.Name,
		Age:            in.Age,
		ParentPhone:    in.ParentPhone,
		ParentEmail:    in.ParentEmail,
		HifzLevel:      in.HifzLevel,
		Status:         "active",
	}
	if err := s.students.Create(ctx, st); err != nil {
		return nil, fmt.Errorf("create student: %w", err)
	}

	tt := "student"
	tid := st.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "enroll_student",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return st, nil
}

func (s *StudentService) ListByHalaqah(ctx context.Context, halaqahID uuid.UUID, limit, offset int) ([]model.Student, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.students.ListByHalaqah(ctx, halaqahID, limit, offset)
}

func (s *StudentService) Transfer(ctx context.Context, actorID, studentID, newHalaqahID, orgID uuid.UUID) error {
	h, err := s.halaqat.GetByID(ctx, newHalaqahID)
	if err != nil {
		return fmt.Errorf("lookup halaqah: %w", err)
	}
	if h.OrganizationID != orgID {
		return ErrCrossTenant
	}
	if err := s.students.Transfer(ctx, studentID, newHalaqahID); err != nil {
		return fmt.Errorf("transfer student: %w", err)
	}
	tt := "student"
	tid := studentID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "transfer_student",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}

func (s *StudentService) Deactivate(ctx context.Context, actorID, id uuid.UUID) error {
	st, err := s.students.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("lookup student: %w", err)
	}
	st.Status = "inactive"
	if err := s.students.Update(ctx, st); err != nil {
		return fmt.Errorf("update student: %w", err)
	}
	tt := "student"
	tid := st.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "deactivate_student",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return nil
}
