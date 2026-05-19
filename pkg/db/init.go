package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

type Table string

var (
	TableBookmarks Table = "bookmarks"
	TableTags      Table = "tags"
	TableRelation  Table = "bookmark_tags"
	TableMetadata  Table = "metadata"
)

func (t Table) Exists(ctx context.Context, r *SQLite) (bool, error) {
	var count int
	err := r.DB.GetContext(
		ctx,
		&count,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?",
		t.String(),
	)
	if err != nil {
		slog.ErrorContext(ctx, "checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}

func (t Table) String() string {
	return string(t)
}

var tables = []Table{
	TableBookmarks,
	TableTags,
	TableRelation,
	TableMetadata,
}

// Init initializes a new database and creates the required tables.
func (r *SQLite) Init(ctx context.Context) error {
	return Migrate(ctx, r)
}

// tableRename renames the temporary table to the specified main table name.
func (r *SQLite) tableRename(ctx context.Context, tx *sqlx.Tx, srcTable, destTable Table) error {
	slog.Debug("renaming table", "from", srcTable, "to", destTable)

	_, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", srcTable, destTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from %q to %q", err, srcTable, destTable)
	}

	return nil
}

// tableCreate creates a new table with the specified name in the SQLite database.
func (r *SQLite) tableCreate(ctx context.Context, tx *sqlx.Tx, name Table, schema string) error {
	slog.DebugContext(ctx, "creating table", "name", name)

	_, err := tx.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// tableDrop drops the specified table from the SQLite database.
func (r *SQLite) tableDrop(ctx context.Context, tx *sqlx.Tx, t Table) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table %q", err, t)
	}

	slog.Debug("dropped table", "name", t)

	return nil
}

// Vacuum rebuilds the database file, repacking it into a minimal amount of
// disk space.
func (r *SQLite) Vacuum(ctx context.Context) error {
	return vacuum(ctx, r)
}

// DropSecure removes all records database.
func (r *SQLite) DropSecure(ctx context.Context) error {
	return drop(ctx, r)
}

// IsInitialized checks if the database is initialized.
func (r *SQLite) IsInitialized(ctx context.Context) bool {
	schemas := []Schema{
		Schemas.Main,
		Schemas.Tags,
		Schemas.Relation,
	}

	allExist := true
	for _, s := range schemas {
		exists, err := tableExists(ctx, r, s.Name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.Name, "error", err)
			return false
		}

		if !exists {
			allExist = false
			slog.Warn("table does not exist", "name", s.Name)
		}
	}

	return allExist
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func tableExists(ctx context.Context, r *SQLite, t Table) (bool, error) {
	var count int
	err := r.DB.GetContext(ctx, &count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", t)
	if err != nil {
		slog.Error("checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}
