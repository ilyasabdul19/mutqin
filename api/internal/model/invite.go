// api/internal/model/invite.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Invite struct {
	bun.BaseModel `bun:"table:invites,alias:i"`

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID  `bun:"organization_id,notnull,type:uuid"`
	Role           string     `bun:"role,notnull"`
	Token          string     `bun:"token,notnull,unique"`
	ExpiresAt      time.Time  `bun:"expires_at,notnull"`
	UsedAt         *time.Time `bun:"used_at"`
	CreatedBy      uuid.UUID  `bun:"created_by,notnull,type:uuid"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
