// api/internal/model/halaqah.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Halaqah struct {
	bun.BaseModel `bun:"table:halaqat,alias:h"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	Name           string     `bun:"name,notnull"`
	TeacherID      *uuid.UUID `bun:"teacher_id,type:uuid"`
	Schedule       []byte     `bun:"schedule,type:jsonb"`
	MaxCapacity    int        `bun:"max_capacity,notnull,nullzero,default:30"`
	Status         string     `bun:"status,notnull,nullzero,default:'active'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
