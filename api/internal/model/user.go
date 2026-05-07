// api/internal/model/user.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type User struct {
	bun.BaseModel `bun:"table:users,alias:u"`

	ID             uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	Phone          *string    `bun:"phone"`
	Email          *string    `bun:"email"`
	Name           string     `bun:"name,notnull"`
	Role           string     `bun:"role,notnull"`
	OrganizationID *uuid.UUID `bun:"organization_id,type:uuid"`
	Status         string     `bun:"status,notnull,nullzero,default:'active'"`
	Language       string     `bun:"language,notnull,nullzero,default:'ar'"`
	CreatedAt      time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
}
