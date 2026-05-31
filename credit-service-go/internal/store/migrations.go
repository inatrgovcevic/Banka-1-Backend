package store

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, db *pgxpool.Pool, migrationsDir string) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS go_schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, file := range files {
		version := filepath.Base(file)

		var exists bool
		err = db.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM go_schema_migrations WHERE version = $1)`,
			version,
		).Scan(&exists)
		if err != nil {
			return err
		}

		if exists {
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, string(sqlBytes))
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO go_schema_migrations (version) VALUES ($1)`,
			version,
		)
		if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}

		err = tx.Commit(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}
