// api/internal/model/attendance.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Attendance struct {
	bun.BaseModel `bun:"table:attendance,alias:a"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	HalaqahID      uuid.UUID  `bun:"halaqah_id,notnull,type:uuid"`
	StudentID      uuid.UUID  `bun:"student_id,notnull,type:uuid"`
	Date           time.Time  `bun:"date,notnull,type:date"`
	Status         string     `bun:"status,notnull"`
	RecordedAt     time.Time  `bun:"recorded_at,notnull,nullzero,default:now()"`
	SyncedAt       *time.Time `bun:"synced_at"`
	ClientID       *string    `bun:"client_id"`
}
