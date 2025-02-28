package repo

import (
	"context"
	"fmt"
	"log"

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
				return fmt.Errorf("creating '%s' table: %w", s.name, err)
			}

			if s.index != "" {
				if _, err := tx.Exec(s.index); err != nil {
					return fmt.Errorf("creating '%s' index: %w", s.name, err)
				}
			}

			if s.trigger != "" {
				if _, err := tx.Exec(s.trigger); err != nil {
					return fmt.Errorf("creating '%s' trigger: %w", s.name, err)
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
			log.Printf("IsInitialized: checking if table exists: %v", err)
			return false
		}
		if !exists {
			allExist = false
			log.Printf("table %s does not exist", s.name)
		}
	}

	return allExist
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func (r *SQLiteRepository) tableExists(t Table) (bool, error) {
	var count int
	err := r.DB.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", t)
	if err != nil {
		log.Printf("error checking if table %s exists: %v", t, err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}

// tableRename renames the temporary table to the specified main table name.
func (r *SQLiteRepository) tableRename(tx *sqlx.Tx, srcTable, destTable Table) error {
	log.Printf("renaming table %s to %s", srcTable, destTable)
	_, err := tx.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", srcTable, destTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from '%s' to '%s'", err, srcTable, destTable)
	}
	log.Printf("renamed table %s to %s\n", srcTable, destTable)

	return nil
}

// tableCreate creates a new table with the specified name in the SQLite database.
func (r *SQLiteRepository) tableCreate(tx *sqlx.Tx, name Table, schema string) error {
	log.Printf("creating table: %s", name)
	_, err := tx.Exec(schema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// tableDrop drops the specified table from the SQLite database.
func (r *SQLiteRepository) tableDrop(tx *sqlx.Tx, t Table) error {
	log.Printf("dropping table: %s", t)

	_, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table '%s'", err, t)
	}

	log.Printf("dropped table: %s\n", t)

	return nil
}

// maintenance performs maintenance tasks on the SQLite repository.
func (r *SQLiteRepository) maintenance() error {
	if err := r.checkSize(r.Cfg.MaxBytesSize); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func (r *SQLiteRepository) resetSQLiteSequence(tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		log.Printf("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		log.Printf("resetting sqlite sequence for table: %s", t)
		if _, err := tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
	}

	return nil
}

// vacuum rebuilds the database file, repacking it into a minimal amount of
// disk space.
func (r *SQLiteRepository) vacuum() error {
	log.Println("vacuuming database")
	_, err := r.DB.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// size returns the size of the database.
func (r *SQLiteRepository) size() (int64, error) {
	var size int64
	err := r.DB.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").
		Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("size: %w", err)
	}

	log.Printf("size of the database: %d bytes\n", size)

	return size, nil
}

// checkSize checks the size of the database.
func (r *SQLiteRepository) checkSize(n int64) error {
	size, err := r.size()
	if err != nil {
		return fmt.Errorf("size: %w", err)
	}
	if size > n {
		return r.vacuum()
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

	return r.vacuum()
}
