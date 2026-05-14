// api/internal/model/sync_conflict.go
package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

type SyncConflict struct {
	bun.BaseModel `bun:"table:sync_conflicts,alias:sc"`
	TenantScoped

	ID             uuid.UUID `bun:"id,pk,type:uuid,nullzero,default:gen_random_uuid()"`
	OrganizationID uuid.UUID `bun:"organization_id,notnull,type:uuid"`
	TableName      string    `bun:"table_name,notnull"`
	RecordID       uuid.UUID `bun:"record_id,notnull,type:uuid"`
	ClientID       *string   `bun:"client_id"`
	ClientData     []byte    `bun:"client_data,type:jsonb,notnull"`
	ServerData     []byte    `bun:"server_data,type:jsonb,notnull"`
	Resolution     string    `bun:"resolution,notnull,nullzero,default:'last_write_wins'"`
	CreatedAt      time.Time `bun:"created_at,notnull,nullzero,default:now()"`
}
