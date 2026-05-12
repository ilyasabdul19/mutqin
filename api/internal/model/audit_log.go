// api/internal/model/audit_log.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type AuditLog struct {
	bun.BaseModel `bun:"table:audit_log,alias:al"`

	ID         uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	ActorID    *uuid.UUID `bun:"actor_id,type:uuid"`
	Action     string     `bun:"action,notnull"`
	TargetType *string    `bun:"target_type"`
	TargetID   *uuid.UUID `bun:"target_id,type:uuid"`
	Details    []byte     `bun:"details,type:jsonb"`
	CreatedAt  time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
