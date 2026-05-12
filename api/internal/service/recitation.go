package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/ilyas/mutqin-api/internal/model"
)

var (
	ErrInvalidRecitationInput = errors.New("service: invalid recitation input")
)

var validTypes = map[string]struct{}{
	"new_hifz": {}, "near_review": {}, "far_review": {},
}
var validGrades = map[string]struct{}{
	"mumtaz": {}, "jayyid_jiddan": {}, "jayyid": {}, "maqbul": {}, "daif": {},
}

type recitationRepoIface interface {
	Create(ctx context.Context, r *model.Recitation) error
	BatchCreate(ctx context.Context, rs []model.Recitation) error
	ListByStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error)
	GetLatestByStudent(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error)
}

type RecitationService struct {
	recs  recitationRepoIface
	audit auditRepo
}

func NewRecitationService(recs recitationRepoIface, audit auditRepo) *RecitationService {
	return &RecitationService{recs: recs, audit: audit}
}

// RecordInput is the validated payload for a single recitation. ClientID is
// optional and used by offline sync (Plan N) to dedupe.
type RecordInput struct {
	StudentID   uuid.UUID
	HalaqahID   uuid.UUID
	Type        string
	SurahNumber int
	AyahFrom    int
	AyahTo      int
	Grade       string
	Notes       *string
	ClientID    *string
}

func validateInput(in RecordInput) error {
	if in.StudentID == uuid.Nil || in.HalaqahID == uuid.Nil {
		return fmt.Errorf("%w: student_id and halaqah_id required", ErrInvalidRecitationInput)
	}
	if _, ok := validTypes[in.Type]; !ok {
		return fmt.Errorf("%w: type must be new_hifz/near_review/far_review", ErrInvalidRecitationInput)
	}
	if _, ok := validGrades[in.Grade]; !ok {
		return fmt.Errorf("%w: grade must be mumtaz/jayyid_jiddan/jayyid/maqbul/daif", ErrInvalidRecitationInput)
	}
	if in.SurahNumber < 1 || in.SurahNumber > 114 {
		return fmt.Errorf("%w: surah_number must be 1..114", ErrInvalidRecitationInput)
	}
	if in.AyahFrom < 1 || in.AyahTo < in.AyahFrom {
		return fmt.Errorf("%w: ayah range invalid", ErrInvalidRecitationInput)
	}
	return nil
}

func (s *RecitationService) Record(ctx context.Context, actorID, orgID uuid.UUID, in RecordInput) (*model.Recitation, error) {
	if err := validateInput(in); err != nil {
		return nil, err
	}
	rec := &model.Recitation{
		OrganizationID: orgID,
		StudentID:      in.StudentID,
		HalaqahID:      in.HalaqahID,
		TeacherID:      actorID,
		Type:           in.Type,
		SurahNumber:    in.SurahNumber,
		AyahFrom:       in.AyahFrom,
		AyahTo:         in.AyahTo,
		Grade:          in.Grade,
		Notes:          in.Notes,
		ClientID:       in.ClientID,
	}
	if err := s.recs.Create(ctx, rec); err != nil {
		return nil, fmt.Errorf("record recitation: %w", err)
	}

	tt := "recitation"
	tid := rec.ID
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "record_recitation",
		TargetType: &tt,
		TargetID:   &tid,
	})
	return rec, nil
}

func (s *RecitationService) RecordBatch(ctx context.Context, actorID, orgID uuid.UUID, ins []RecordInput) ([]model.Recitation, error) {
	if len(ins) == 0 {
		return nil, nil
	}
	recs := make([]model.Recitation, 0, len(ins))
	for i, in := range ins {
		if err := validateInput(in); err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		recs = append(recs, model.Recitation{
			OrganizationID: orgID,
			StudentID:      in.StudentID,
			HalaqahID:      in.HalaqahID,
			TeacherID:      actorID,
			Type:           in.Type,
			SurahNumber:    in.SurahNumber,
			AyahFrom:       in.AyahFrom,
			AyahTo:         in.AyahTo,
			Grade:          in.Grade,
			Notes:          in.Notes,
			ClientID:       in.ClientID,
		})
	}
	if err := s.recs.BatchCreate(ctx, recs); err != nil {
		return nil, fmt.Errorf("batch record: %w", err)
	}

	tt := "recitation"
	_ = s.audit.Create(ctx, &model.AuditLog{
		ActorID:    &actorID,
		Action:     "record_recitation_batch",
		TargetType: &tt,
	})
	return recs, nil
}

func (s *RecitationService) ListForStudent(ctx context.Context, studentID uuid.UUID, limit, offset int) ([]model.Recitation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.recs.ListByStudent(ctx, studentID, limit, offset)
}

func (s *RecitationService) GetLatest(ctx context.Context, studentID uuid.UUID) (*model.Recitation, error) {
	return s.recs.GetLatestByStudent(ctx, studentID)
}
