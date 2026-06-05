package store

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the subset of *pgxpool.Pool used by the stores. Extracting it as an
// interface lets tests inject lightweight fakes without a live Postgres, while
// the production constructors continue to accept a concrete *pgxpool.Pool (which
// satisfies this interface).
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}
