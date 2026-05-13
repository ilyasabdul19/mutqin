// api/internal/model/announcement.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Announcement struct {
	bun.BaseModel `bun:"table:announcements,alias:a"`
	TenantScoped

	ID             uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
	Title          string    `bun:"title,notnull"`
	Body           string    `bun:"body,notnull"`
	CreatedAt      time.Time `bun:"created_at,notnull,nullzero,default:now()"`
}
