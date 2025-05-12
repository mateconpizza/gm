package repo

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
)

type tableSchema struct {
	name    Table
	sql     string
	trigger string
	index   string
}

// tablesAnd returns all tables and their schema.
func tablesAndSchema() []tableSchema {
	return []tableSchema{
		schemaMain, schemaTags, schemaRelation,
	}
}

// Init initializes a new database and creates the required tables.
func (r *SQLiteRepository) Init() error {
	return r.withTx(context.Background(), func(tx *sqlx.Tx) error {
		for _, s := range tablesAndSchema() {
			if err := r.tableCreate(tx, s.name, s.sql); err != nil {
				return fmt.Errorf("creating %q table: %w", s.name, err)
			}

			if s.index != "" {
				if _, err := tx.Exec(s.index); err != nil {
					return fmt.Errorf("creating %q index: %w", s.name, err)
				}
			}

			if s.trigger != "" {
				if _, err := tx.Exec(s.trigger); err != nil {
					return fmt.Errorf("creating %q trigger: %w", s.name, err)
				}
			}
		}

		return nil
	})
}

// IsInitialized returns true if the database is initialized.
func (r *SQLiteRepository) IsInitialized() bool {
	allExist := true
	for _, s := range tablesAndSchema() {
		exists, err := r.tableExists(s.name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.name, "error", err)
			return false
		}
		if !exists {
			allExist = false
			slog.Warn("table does not exist", "name", s.name)
		}
	}

	return allExist
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func (r *SQLiteRepository) tableExists(t Table) (bool, error) {
	var count int
	err := r.DB.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", t)
	if err != nil {
		slog.Error("checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}

// tableRename renames the temporary table to the specified main table name.
func (r *SQLiteRepository) tableRename(tx *sqlx.Tx, srcTable, destTable Table) error {
	slog.Info("renaming table", "from", srcTable, "to", destTable)
	_, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", srcTable, destTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from %q to %q", err, srcTable, destTable)
	}

	return nil
}

// tableCreate creates a new table with the specified name in the SQLite database.
func (r *SQLiteRepository) tableCreate(tx *sqlx.Tx, name Table, schema string) error {
	slog.Debug("creating table", "name", name)
	_, err := tx.Exec(schema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// tableDrop drops the specified table from the SQLite database.
func (r *SQLiteRepository) tableDrop(tx *sqlx.Tx, t Table) error {
	_, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table %q", err, t)
	}

	slog.Debug("dropped table", "name", t)

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func (r *SQLiteRepository) resetSQLiteSequence(tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		slog.Warn("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		slog.Debug("resetting sqlite sequence", "table", t)
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
	}

	return nil
}

// Vacuum rebuilds the database file, repacking it into a minimal amount of
// disk space.
func (r *SQLiteRepository) Vacuum() error {
	slog.Debug("vacuuming database")
	_, err := r.DB.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// DropSecure removes all records database.
func (r *SQLiteRepository) DropSecure(ctx context.Context) error {
	tts := tablesAndSchema()
	tables := make([]Table, 0, len(tts))
	for _, t := range tts {
		tables = append(tables, t.name)
	}
	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteAll(ctx, tables...); err != nil {
			return fmt.Errorf("%w", err)
		}

		return r.resetSQLiteSequence(tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.Vacuum()
}
