package database

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MigrationFile represents a parsed migration file with its version number,
// filename, and SQL content.
type MigrationFile struct {
	Version  int
	Filename string
	SQL      string
}

const schemaMigrationsLockID int64 = 8_034_219_741

type migrationDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// RunMigrations applies all pending .up.sql migrations from the given embed.FS.
// It creates a schema_versions table to track which migrations have been applied,
// parses migration filenames to extract version numbers, and applies each
// pending migration within its own transaction.
//
// The dir parameter specifies the subdirectory within the fs.FS that contains
// the migration files.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, dir string) (err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquiring migration connection: %w", err)
	}
	defer conn.Release()

	if err := lockMigrations(ctx, conn); err != nil {
		return fmt.Errorf("acquiring migration lock: %w", err)
	}
	defer func() {
		if unlockErr := unlockMigrations(conn); unlockErr != nil {
			if err != nil {
				err = fmt.Errorf("%w; releasing migration lock: %v", err, unlockErr)
				return
			}
			err = fmt.Errorf("releasing migration lock: %w", unlockErr)
		}
	}()

	if err := ensureSchemaVersionsTable(ctx, conn); err != nil {
		return fmt.Errorf("ensuring schema_versions table: %w", err)
	}

	migrations, err := loadMigrations(fsys, dir)
	if err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	applied, err := getAppliedVersions(ctx, conn)
	if err != nil {
		return fmt.Errorf("getting applied versions: %w", err)
	}

	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}

		if err := applyMigration(ctx, conn, m); err != nil {
			return fmt.Errorf("applying migration %s (version %d): %w", m.Filename, m.Version, err)
		}
		applied[m.Version] = true
	}

	return nil
}

func lockMigrations(ctx context.Context, db migrationDB) error {
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", schemaMigrationsLockID)
	return err
}

func unlockMigrations(db migrationDB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, "SELECT pg_advisory_unlock($1)", schemaMigrationsLockID)
	return err
}

// ensureSchemaVersionsTable creates the schema_versions table if it does not
// already exist.
func ensureSchemaVersionsTable(ctx context.Context, db migrationDB) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS public.schema_versions (
			version    INT PRIMARY KEY,
			filename   TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`)
	return err
}

// loadMigrations reads all .up.sql files from the specified directory within
// the filesystem and returns them sorted by version number.
func loadMigrations(fsys fs.FS, dir string) ([]MigrationFile, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("reading migrations directory %q: %w", dir, err)
	}

	var migrations []MigrationFile
	seenVersions := make(map[int]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		version, err := parseVersion(name)
		if err != nil {
			return nil, fmt.Errorf("parsing version from %q: %w", name, err)
		}
		if existing, ok := seenVersions[version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d: %s and %s", version, existing, name)
		}
		seenVersions[version] = name

		content, err := fs.ReadFile(fsys, filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading migration file %q: %w", name, err)
		}

		migrations = append(migrations, MigrationFile{
			Version:  version,
			Filename: name,
			SQL:      string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// parseVersion extracts the numeric version prefix from a migration filename.
// For example, "001_initial_schema.up.sql" returns 1.
func parseVersion(filename string) (int, error) {
	// Split on the first underscore to get the version prefix.
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) < 2 {
		return 0, fmt.Errorf("migration filename %q must be in format NNN_description.up.sql", filename)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid version number in %q: %w", filename, err)
	}

	if version <= 0 {
		return 0, fmt.Errorf("version number must be positive, got %d in %q", version, filename)
	}

	return version, nil
}

// getAppliedVersions returns a set of migration versions that have already
// been applied.
func getAppliedVersions(ctx context.Context, db migrationDB) (map[int]bool, error) {
	rows, err := db.Query(ctx, "SELECT version FROM public.schema_versions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}

// applyMigration runs a single migration within a transaction and records the
// version in schema_versions.
func applyMigration(ctx context.Context, db migrationDB, m MigrationFile) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() {
		// Rollback is a no-op if the tx has already been committed.
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("executing SQL: %w", err)
	}

	if _, err := tx.Exec(ctx,
		"INSERT INTO public.schema_versions (version, filename) VALUES ($1, $2)",
		m.Version, m.Filename,
	); err != nil {
		return fmt.Errorf("recording version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}
