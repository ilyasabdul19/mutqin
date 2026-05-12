// api/internal/model/registration.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Registration struct {
	bun.BaseModel `bun:"table:registrations,alias:r"`
	TenantScoped

	ID             uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
	ChildName      string    `bun:"child_name,notnull"`
	ChildAge       *int      `bun:"child_age"`
	ParentPhone    *string   `bun:"parent_phone"`
	ParentEmail    *string   `bun:"parent_email"`
	HifzLevel      *string   `bun:"hifz_level"`
	Status         string    `bun:"status,notnull,nullzero,default:'pending'"`
	SubmittedAt    time.Time `bun:"submitted_at,notnull,nullzero,default:now()"`
}
