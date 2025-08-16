package db

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

// tablesAndSchemas all tables and their schema.
var tablesAndSchemas = []Schema{
	schemaMain,
	schemaTags,
	schemaRelation,
}

// Init initializes a new database and creates the required tables.
func (r *SQLite) Init(ctx context.Context) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, s := range tablesAndSchemas {
			if err := r.tableCreate(ctx, tx, s.Name, s.SQL); err != nil {
				return fmt.Errorf("creating %q table: %w", s.Name, err)
			}

			if s.Index != "" {
				if _, err := tx.ExecContext(ctx, s.Index); err != nil {
					return fmt.Errorf("creating %q index: %w", s.Name, err)
				}
			}

			if len(s.Trigger) > 0 {
				for _, t := range s.Trigger {
					if _, err := tx.ExecContext(ctx, t); err != nil {
						return fmt.Errorf("creating %q trigger: %w", s.Name, err)
					}
				}
			}
		}

		return nil
	})
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
	slog.Debug("creating table", "name", name)

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

// tableExists checks whether a table with the specified name exists in the SQLite database.
func tableExists(r *SQLite, t Table) (bool, error) {
	var count int
	err := r.DB.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", t)
	if err != nil {
		slog.Error("checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}
