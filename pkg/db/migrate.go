package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

var ErrInvalidMigrationFilename = errors.New("invalid migration filename")

//go:embed migrations/*.sql
var migrationFS embed.FS

type Migration struct {
	Version int
	Name    string
	File    string
	SQL     string
}

func Migrate(ctx context.Context, r *SQLite) error {
	slog.DebugContext(ctx, "starting database migration")

	migrations, err := LoadMigrations()
	if err != nil {
		slog.ErrorContext(ctx, "failed to load migrations", "error", err)
		return err
	}
	slog.DebugContext(ctx, "loaded migrations", "count", len(migrations))

	current, err := CurrentSchemaVersion(ctx, r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get current version", "error", err)
		return err
	}
	slog.DebugContext(ctx, "current database version", "version", current)

	pendingCount := 0
	for _, m := range migrations {
		if m.Version <= current {
			slog.DebugContext(ctx, "skipping migration", "version", m.Version, "reason", "already applied")
			continue
		}
		pendingCount++
	}

	if pendingCount == 0 {
		slog.DebugContext(ctx, "database is up to date", "current_version", current)
		return nil
	}

	slog.DebugContext(ctx, "applying pending migrations", "count", pendingCount, "from_version", current)

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}

		slog.DebugContext(ctx, "applying migration", "version", m.Version, "name", m.Name)

		if err := applyMigration(ctx, r, m); err != nil {
			slog.ErrorContext(ctx, "migration failed",
				"version", m.Version,
				"name", m.Name,
				"error", err)
			return err
		}

		slog.DebugContext(ctx, "migration applied successfully", "version", m.Version, "name", m.Name)
	}

	slog.DebugContext(ctx, "database migration completed", "final_version", current+len(migrations))
	return nil
}

func LoadMigrations() ([]Migration, error) {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return nil, err
	}

	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		if filepath.Ext(name) != ".sql" {
			continue
		}

		version, migrationName, err := parseMigrationFilename(name)
		if err != nil {
			return nil, err
		}

		path := filepath.Join("migrations", name)

		content, err := migrationFS.ReadFile(path)
		if err != nil {
			return nil, err
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    migrationName,
			File:    name,
			SQL:     string(content),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func SQLiteVersion(ctx context.Context, r *SQLite) (string, error) {
	var ver string
	err := r.DB.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&ver)
	if err != nil {
		return "", err
	}

	return ver, nil
}

func CurrentSchemaVersion(ctx context.Context, r *SQLite) (int, error) {
	var ver int
	err := r.DB.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&ver)
	if err != nil {
		return -1, err
	}

	return ver, nil
}

func parseMigrationFilename(name string) (version int, migration string, err error) {
	base := strings.TrimSuffix(name, ".sql")

	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("%w: %s", ErrInvalidMigrationFilename, name)
	}

	version, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", err
	}

	return version, parts[1], nil
}

func applyMigration(ctx context.Context, db *SQLite, m Migration) error {
	return db.WithTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf(
				"apply migration %04d_%s: %w",
				m.Version,
				m.Name,
				err,
			)
		}

		query := "PRAGMA user_version = " + strconv.Itoa(m.Version)

		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf(
				"set schema version %d: %w",
				m.Version,
				err,
			)
		}

		return nil
	})
}
