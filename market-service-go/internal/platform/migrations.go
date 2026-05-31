package platform

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

func RunMigrations(ctx context.Context, db *pgxpool.Pool, dir string) error {
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS go_schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT now()
		)`); err != nil {
		return err
	}

	if err := baselineExistingJavaSchema(ctx, db, dir); err != nil {
		return err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		var applied bool
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM go_schema_migrations WHERE filename = $1)`, file).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return err
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO go_schema_migrations(filename) VALUES ($1)`, file); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}

	return nil
}

func baselineExistingJavaSchema(ctx context.Context, db *pgxpool.Pool, dir string) error {
	var tracked int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM go_schema_migrations`).Scan(&tracked); err != nil {
		return err
	}
	if tracked > 0 {
		return nil
	}

	var hasExchangeRate, hasListing bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.exchange_rate') IS NOT NULL`).Scan(&hasExchangeRate); err != nil {
		return err
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.listing') IS NOT NULL`).Scan(&hasListing); err != nil {
		return err
	}
	if !hasExchangeRate || !hasListing {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		if _, err := tx.Exec(ctx, `INSERT INTO go_schema_migrations(filename) VALUES ($1) ON CONFLICT DO NOTHING`, entry.Name()); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}
	return tx.Commit(ctx)
}
