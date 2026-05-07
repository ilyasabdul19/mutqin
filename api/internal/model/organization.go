// api/internal/model/organization.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type Organization struct {
	bun.BaseModel `bun:"table:organizations,alias:o"`

	ID          uuid.UUID  `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	Name        string     `bun:"name,notnull"`
	Slug        string     `bun:"slug,notnull,unique"`
	City        *string    `bun:"city"`
	Country     string     `bun:"country,notnull,nullzero,default:'SO'"`
	Tier        string     `bun:"tier,notnull,nullzero,default:'free'"`
	Status      string     `bun:"status,notnull,nullzero,default:'active'"`
	LogoURL     *string    `bun:"logo_url"`
	Description *string    `bun:"description"`
	Schedule    []byte     `bun:"schedule,type:jsonb"`
	CreatedAt   time.Time  `bun:"created_at,notnull,nullzero,default:now()"`
	UpdatedAt   time.Time  `bun:"updated_at,notnull,nullzero,default:now()"`
}
