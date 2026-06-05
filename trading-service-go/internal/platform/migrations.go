package platform

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationDB is the subset of *pgxpool.Pool used by RunMigrations and
// baselineExistingJavaSchema. *pgxpool.Pool satisfies it; tests inject a fake.
type migrationDB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RunMigrations applies the .sql files in dir in lexical order, tracking applied
// files in go_schema_migrations. It mirrors market-service-go's runner with two
// trading-specific behaviors:
//
//   - baselineExistingJavaSchema: on a database Java Liquibase already provisioned
//     (the coexistence / cut-over case — Java owned the `trading` schema before
//     trading-service-go took it over), every migration is recorded as applied
//     WITHOUT running it, so the Go baseline never collides with the existing
//     tables. On a fresh database the migrations run normally and create the schema.
//   - dev-seed gating: files whose name contains "devseed" run only when
//     LIQUIBASE_CONTEXTS contains "dev" (the same dev signal the rest of the stack
//     uses; compose default is "dev"). Keeps demo fixtures out of a prod database.
func RunMigrations(ctx context.Context, db *pgxpool.Pool, dir string) error {
	return runMigrations(ctx, db, dir)
}

func runMigrations(ctx context.Context, db migrationDB, dir string) error {
	if _, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.go_schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("create table step failed: %w", err)
	}

	if err := baselineExistingJavaSchema(ctx, db, dir); err != nil {
		return fmt.Errorf("baseline step failed: %w", err)
	}

	devSeed := devSeedEnabled()

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
		if err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.go_schema_migrations WHERE filename = $1)`, file).Scan(&applied); err != nil {
			return fmt.Errorf("check loop step failed for %s: %w", file, err)
		}
		if applied {
			continue
		}
		if isDevSeed(file) && !devSeed {
			// Skip dev-only fixtures outside a dev context. Do NOT record them as
			// applied, so flipping LIQUIBASE_CONTEXTS to dev later still applies them.
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			return fmt.Errorf("read file failed for %s: %w", file, err)
		}
		tx, err := db.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx failed for %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("execute sql failed for %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.go_schema_migrations(filename) VALUES ($1)`, file); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("insert migration tracking failed for %s: %w", file, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit tx failed for %s: %w", file, err)
		}
	}

	return nil
}

// isDevSeed reports whether a migration file is a dev-only fixture (gated by
// devSeedEnabled). The cut-over seed file is named *_devseed_*.sql.
func isDevSeed(filename string) bool {
	return strings.Contains(filename, "devseed")
}

// devSeedEnabled mirrors the Liquibase context:dev gate the Java services use:
// dev fixtures apply when LIQUIBASE_CONTEXTS contains "dev". The compose default
// is "dev", so fixtures apply locally unless an operator sets a non-dev context
// (e.g. LIQUIBASE_CONTEXTS=prod) for a production database.
func devSeedEnabled() bool {
	ctxs := strings.ToLower(strings.TrimSpace(os.Getenv("LIQUIBASE_CONTEXTS")))
	if ctxs == "" {
		ctxs = "dev"
	}
	for _, c := range strings.Split(ctxs, ",") {
		if strings.TrimSpace(c) == "dev" {
			return true
		}
	}
	return false
}

// baselineExistingJavaSchema records every migration as already-applied (without
// executing it) when go_schema_migrations is empty AND the Java Liquibase schema
// is already present (sentinel tables portfolio + orders). This is the
// coexistence / cut-over path: the `trading` database was created and owned by
// Java Liquibase, so the Go baseline must not try to recreate those tables. On a
// fresh database (no sentinel tables) it is a no-op and the migrations run normally.
func baselineExistingJavaSchema(ctx context.Context, db migrationDB, dir string) error {
	var tracked int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM public.go_schema_migrations`).Scan(&tracked); err != nil {
		return fmt.Errorf("baseline check count failed: %w", err)
	}
	if tracked > 0 {
		return nil
	}

	var hasPortfolio, hasOrders bool
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.portfolio') IS NOT NULL`).Scan(&hasPortfolio); err != nil {
		return fmt.Errorf("baseline check portfolio failed: %w", err)
	}
	if err := db.QueryRow(ctx, `SELECT to_regclass('public.orders') IS NOT NULL`).Scan(&hasOrders); err != nil {
		return fmt.Errorf("baseline check orders failed: %w", err)
	}
	if !hasPortfolio || !hasOrders {
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
		if _, err := tx.Exec(ctx, `INSERT INTO public.go_schema_migrations(filename) VALUES ($1) ON CONFLICT DO NOTHING`, entry.Name()); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
	}
	return tx.Commit(ctx)
}
