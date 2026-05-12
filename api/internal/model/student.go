// api/internal/model/student.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Student struct {
	bun.BaseModel `bun:"table:students,alias:s"`
	TenantScoped

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	HalaqahID      *uuid.UUID `bun:"halaqah_id,type:uuid"`
	Name           string     `bun:"name,notnull"`
	Age            *int       `bun:"age"`
	ParentPhone    *string    `bun:"parent_phone"`
	ParentEmail    *string    `bun:"parent_email"`
	HifzLevel      *string    `bun:"hifz_level"`
	Status         string     `bun:"status,notnull,nullzero,default:'active'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
