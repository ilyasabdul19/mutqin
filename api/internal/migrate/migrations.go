// api/internal/migrate/migrations.go
package migrate

import (
	"github.com/uptrace/bun/migrate"
)

// Migrations is the global registry. Migration files self-register via init().
var Migrations = migrate.NewMigrations()
