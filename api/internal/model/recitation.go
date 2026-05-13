package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Recitation struct {
	bun.BaseModel `bun:"table:recitations,alias:r"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	StudentID      uuid.UUID  `bun:"student_id,notnull,type:uuid"`
	HalaqahID      uuid.UUID  `bun:"halaqah_id,notnull,type:uuid"`
	TeacherID      uuid.UUID  `bun:"teacher_id,notnull,type:uuid"`
	Type           string     `bun:"type,notnull"`
	SurahNumber    int        `bun:"surah_number,notnull"`
	AyahFrom       int        `bun:"ayah_from,notnull"`
	AyahTo         int        `bun:"ayah_to,notnull"`
	Grade          string     `bun:"grade,notnull"`
	Notes          *string    `bun:"notes"`
	RecordedAt     time.Time  `bun:"recorded_at,notnull,nullzero,default:now()"`
	SyncedAt       *time.Time `bun:"synced_at"`
	ClientID       *string    `bun:"client_id"`
}
