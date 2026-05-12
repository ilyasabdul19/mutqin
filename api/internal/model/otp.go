// api/internal/model/otp.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type OtpCode struct {
	bun.BaseModel `bun:"table:otp_codes,alias:oc"`

	ID        uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	Email     string    `bun:"email,notnull"`
	Code      string    `bun:"code,notnull"`
	ExpiresAt time.Time `bun:"expires_at,notnull"`
	Used      bool      `bun:"used,notnull,default:false"`
	CreatedAt time.Time `bun:"created_at,notnull,nullzero,default:now()"`
}
