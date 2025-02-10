package repo

import (
	"context"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

func (r *SQLiteRepository) tablesAndSchema() map[Table]string {
	return map[Table]string{
		r.Cfg.Tables.Main:               tableMainSchema,
		r.Cfg.Tables.Deleted:            tableMainSchema,
		r.Cfg.Tables.Tags:               tableTagsSchema,
		r.Cfg.Tables.RecordsTags:        tableBookmarkTagsSchema,
		r.Cfg.Tables.RecordsTagsDeleted: tableBookmarkTagsSchema,
	}
}

// Init initializes a new database and creates the required tables.
func (r *SQLiteRepository) Init() error {
	tables := r.tablesAndSchema()
	return r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		for table, schema := range tables {
			if err := r.tableCreate(tx, table, schema); err != nil {
				return fmt.Errorf("Init: creating '%s' table: %w", table, err)
			}
		}

		return nil
	})
}

// IsInitialized returns true if the database is initialized.
func (r *SQLiteRepository) IsInitialized() bool {
	allExist := true
	for t := range r.tablesAndSchema() {
		exists, err := r.tableExists(t)
		if err != nil {
			log.Printf("IsInitialized: checking if table exists: %v", err)
			return false
		}
		if !exists {
			allExist = false
			log.Printf("table %s does not exist", t)
		}
	}

	return allExist
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func (r *SQLiteRepository) tableExists(t Table) (bool, error) {
	q := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?"
	var count int
	err := r.DB.Get(&count, q, t)
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
func (r *SQLiteRepository) tableCreate(tx *sqlx.Tx, s Table, schema string) error {
	log.Printf("creating table: %s", s)
	tableSchema := fmt.Sprintf(schema, s)

	_, err := tx.Exec(tableSchema)
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

	if r.IsInitialized() {
		if err := removeUnusedTags(r); err != nil {
			return fmt.Errorf("%w", err)
		}
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

// IsEmpty returns true if the database is empty.
func (r *SQLiteRepository) IsEmpty(tables ...Table) bool {
	for _, t := range tables {
		if r.maxID(t) > 0 {
			return false
		}
	}

	return true
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
	tables := []Table{
		r.Cfg.Tables.Main,
		r.Cfg.Tables.Deleted,
		r.Cfg.Tables.Tags,
		r.Cfg.Tables.RecordsTags,
		r.Cfg.Tables.RecordsTagsDeleted,
	}

	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
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
