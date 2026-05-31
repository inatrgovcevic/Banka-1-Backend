package platform

import (
	"context"

	gpdb "banka1/go-platform/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPostgres delegates to go-platform/db.OpenPool so pool tuning, the startup
// ping, and behavior stay identical across the Go services. As of the P8 cut-over
// this service owns the `trading` schema and runs its own migrations (see
// RunMigrations); on a database Java Liquibase already provisioned they baseline-skip.
func OpenPostgres(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return gpdb.OpenPool(ctx, databaseURL)
}
