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
	log := r.logger.With("fn", "db-migrate", "db", r.Name())

	ok, err := needsMigration(ctx, r, ms)
	if err != nil {
		return err
	}
	if !ok {
		log.Debug("no migration needed")
		return nil
	}

	current, err := r.CurrentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	var pending []Migration
	for _, m := range ms {
		if m.Version > current {
			pending = append(pending, m)
		}
	}

	if len(pending) == 0 {
		log.Debug("database is up to date", "version", current)
		return nil
	}

	log.Debug("applying pending migrations", "count", len(pending), "from_version", current)

	finalVersion := current
	for _, m := range pending {
		mlog := log.With("version", m.Version, "name", m.Name)

		mlog.Debug("applying migration")

		if err := applyMigration(ctx, r, m); err != nil {
			mlog.Debug("migration failed", "error", err)
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}

		mlog.Debug("migration applied")
		finalVersion = m.Version
	}

	log.Debug("migration completed", "from_version", current, "final_version", finalVersion)
	return nil
}

func LoadMigrations() ([]Migration, error) {
	return loadMigrations(migrationFS)
}

func (r *SQLite) SQLiteVersion(ctx context.Context) (string, error) {
	var ver string
	err := r.DB.QueryRowContext(ctx, `SELECT sqlite_version()`).Scan(&ver)
	if err != nil {
		return "", fmt.Errorf("getting sqlite version: %w", err)
	}

	r.logger.Debug("sqlite version", "fn", "db-sqlite-version", "version", ver)

	return ver, nil
}

func (r *SQLite) CurrentSchemaVersion(ctx context.Context) (int, error) {
	var ver int
	err := r.DB.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&ver)
	if err != nil {
		return -1, fmt.Errorf("getting schema version: %w", err)
	}

	r.logger.Debug("schema version", "fn", "db-schema-version", "db", r.Name(), "version", ver)

	return ver, nil
}

func needsMigration(ctx context.Context, r *SQLite, ms []Migration) (bool, error) {
	current, err := r.CurrentSchemaVersion(ctx)
	if err != nil {
		return false, err
	}

	latest := latestMigrationVersion(ms)

	return current < latest, nil
}

func latestMigrationVersion(ms []Migration) int {
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
