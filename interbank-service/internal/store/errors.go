// Package store provides Postgres CRUD for interbank-service domain tables.
// Concurrency control: explicit @Version-style optimistic locking via
// `UPDATE ... SET version = version + 1 WHERE id = $1 AND version = $2`
// and checking RowsAffected() == 1 to detect conflicts.
package store

import "errors"

// ErrOptimisticLockConflict signals that an UPDATE with @Version predicate
// affected 0 rows — another writer changed the row in the meantime.
var ErrOptimisticLockConflict = errors.New("store: optimistic lock conflict")

// ErrNotFound signals a missing row (point lookup that returned 0 rows).
var ErrNotFound = errors.New("store: row not found")
