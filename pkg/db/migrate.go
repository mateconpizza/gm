package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
)

type Migrator interface {
	CurrentVersion(ctx context.Context) (int, error)
	Apply(ctx context.Context, m Migration) error
}

//go:embed migrations/*.sql
var migrationFS embed.FS

type Migration struct {
	Version int
	Name    string
	File    string
	SQL     string
}

func Migrate(ctx context.Context, r *SQLite, ms []Migration) error {
	ok, err := NeedsMigration(ctx, r, ms)
	if err != nil {
		return err
	}
	if !ok {
		slog.DebugContext(ctx, "no migration need it")
		return nil
	}

	slog.DebugContext(ctx, "starting database migration")

	current, err := CurrentSchemaVersion(ctx, r)
	if err != nil {
		slog.ErrorContext(ctx, "failed to get current version", "error", err)
		return err
	}
	slog.DebugContext(ctx, "current database version", "version", current)

	pendingCount := 0
	for _, m := range ms {
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

	finalVersion := current

	for _, m := range ms {
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
		finalVersion = m.Version
	}

	slog.DebugContext(ctx, "database migration completed", "final_version", finalVersion)
	return nil
}

func LoadMigrations() ([]Migration, error) {
	return loadMigrations(migrationFS)
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

func NeedsMigration(ctx context.Context, r *SQLite, ms []Migration) (bool, error) {
	current, err := CurrentSchemaVersion(ctx, r)
	if err != nil {
		return false, err
	}

	latest := LatestMigrationVersion(ms)

	return current < latest, nil
}

func LatestMigrationVersion(ms []Migration) int {
	if len(ms) == 0 {
		return 0
	}

	return ms[len(ms)-1].Version
}

func loadMigrations(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, "migrations")
	if err != nil {
		return nil, err
	}

	ms := make([]Migration, 0, len(entries))

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

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return nil, err
		}

		ms = append(ms, Migration{
			Version: version,
			Name:    migrationName,
			File:    name,
			SQL:     string(content),
		})
	}

	sort.Slice(ms, func(i, j int) bool {
		return ms[i].Version < ms[j].Version
	})

	if err := validateMigrations(ms); err != nil {
		return nil, err
	}

	slog.Debug("loaded migrations", "count", len(ms))

	return ms, nil
}

func parseMigrationFilename(name string) (version int, migration string, err error) {
	base := strings.TrimSuffix(name, ".sql")

	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("%w: %s", ErrMigrationInvalidFilename, name)
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

func validateMigrations(ms []Migration) error {
	if err := validateDuplication(ms); err != nil {
		return err
	}

	return validateMigrationOrder(ms)
}

func validateDuplication(ms []Migration) error {
	seen := map[int]string{}

	for _, m := range ms {
		if prev, ok := seen[m.Version]; ok {
			return fmt.Errorf(
				"%w version %04d: %s and %s",
				ErrMigrationDuplicate,
				m.Version,
				prev,
				m.File,
			)
		}

		seen[m.Version] = m.File
	}

	return nil
}

func validateMigrationOrder(ms []Migration) error {
	for i := 1; i < len(ms); i++ {
		expected := ms[i-1].Version + 1

		if ms[i].Version != expected {
			return fmt.Errorf(
				"%w: expected %04d, got %04d",
				ErrMigrationGap,
				expected,
				ms[i].Version,
			)
		}
	}

	return nil
}
