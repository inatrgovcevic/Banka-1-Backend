package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrNotFound is returned by store methods when the requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrOptimisticLockConflict is returned by UpdateOptimistic when the version
// predicate does not match (another writer has already incremented the version).
var ErrOptimisticLockConflict = errors.New("store: optimistic lock conflict")

// IsUniqueViolation reports whether err is a PostgreSQL unique-constraint
// violation (SQLSTATE 23505). It unwraps through the error chain.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
