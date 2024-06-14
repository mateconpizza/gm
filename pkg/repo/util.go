package repo

import (
	"fmt"
	"log"
	"strconv"

	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/pkg/format"
)

// reorderIDs reorders the IDs in the specified table.
func (r *SQLiteRepository) reorderIDs(tableName string) error {
	// FIX: Everytime we re-order IDs, the db's size gets bigger
	// It's a bad implementation? (but it works)
	// Maybe use 'VACUUM' command? it is safe?
	bs, err := r.GetAll(tableName)
	if err != nil {
		return err
	}

	if len(*bs) == 0 {
		return nil
	}

	log.Printf("reordering IDs in table: %s", tableName)
	tempTable := fmt.Sprintf("temp_%s", tableName)
	if err := r.TableCreate(tempTable, TableMainSchema); err != nil {
		return err
	}

	if err := r.insertBulk(tempTable, bs); err != nil {
		return err
	}

	if err := r.tableDrop(tableName); err != nil {
		return err
	}

	return r.tableRename(tempTable, tableName)
}

// configure configures the repository
func (r *SQLiteRepository) configure(c *SQLiteConfig) error {
	// FIX: when creating a new db, this gets in the way
	// Use this function after checking that the DB is initialized
	/* if exists, _ := r.tableExists(c.GetTableMain()); !exists {
		return fmt.Errorf("%w: '%s'", ErrDBNotInitialized, c.GetName())
	} */
	if err := r.checkSize(c.GetMaxSizeBytes()); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// maintenance
func (r *SQLiteRepository) maintenance(c *SQLiteConfig) error {
	if err := r.checkSize(c.GetMaxSizeBytes()); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// RecordExists checks whether the specified record exists in the SQLite database.
func (r *SQLiteRepository) RecordExists(tableName, column, target string) bool {
	var recordCount int

	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s=?", tableName, column)

	if err := r.DB.QueryRow(sqlQuery, target).Scan(&recordCount); err != nil {
		log.Fatal(err)
	}

	return recordCount > 0
}

// IsInitialized returns true if the database is initialized.
func (r *SQLiteRepository) IsInitialized(tableName string) bool {
	tExists, _ := r.tableExists(tableName)
	return tExists
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func (r *SQLiteRepository) tableExists(t string) (bool, error) {
	query := "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?"

	var count int
	if err := r.DB.QueryRow(query, t).Scan(&count); err != nil {
		log.Printf("table %s does not exist", t)
		return false, fmt.Errorf("%w: checking if table exists", err)
	}

	log.Printf("table '%s' exists: %v", t, count > 0)

	return count > 0, nil
}

// tableRename renames the temporary table to the specified main table name.
func (r *SQLiteRepository) tableRename(tempTable, mainTable string) error {
	log.Printf("renaming table %s to %s", tempTable, mainTable)

	_, err := r.DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tempTable, mainTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from '%s' to '%s'", err, tempTable, mainTable)
	}

	log.Printf("renamed table %s to %s\n", tempTable, mainTable)

	return nil
}

// TableCreate creates a new table with the specified name in the SQLite database.
func (r *SQLiteRepository) TableCreate(name, schema string) error {
	log.Printf("creating table: %s", name)
	tableSchema := fmt.Sprintf(schema, name)

	_, err := r.DB.Exec(tableSchema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// tableDrop drops the specified table from the SQLite database.
func (r *SQLiteRepository) tableDrop(t string) error {
	log.Printf("dropping table: %s", t)

	_, err := r.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table '%s'", err, t)
	}

	log.Printf("dropped table: %s\n", t)

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func (r *SQLiteRepository) resetSQLiteSequence(t string) error {
	if _, err := r.DB.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
		return fmt.Errorf("resetting sqlite sequence: %w", err)
	}

	return nil
}

// vacuum rebuilds the database file, repacking it into a minimal amount of disk space.
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
	err := r.DB.QueryRow("SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()").Scan(&size)
	if err != nil {
		return 0, fmt.Errorf("size: %w", err)
	}

	log.Printf("size of the database: %d bytes\n", size)
	return size, nil
}

// IsEmpty returns true if the database is empty.
func (r *SQLiteRepository) IsEmpty(main, deleted string) bool {
	return r.GetMaxID(main) == 0 && r.GetMaxID(deleted) == 0
}

// checkSize checks the size of the database.
func (r *SQLiteRepository) checkSize(n int64) error {
	log.Println("checking database size")
	size, err := r.size()
	if err != nil {
		return fmt.Errorf("size: %w", err)
	}
	if size > n {
		return r.vacuum()
	}

	return nil
}

// DropSecure removes all records database
func (r *SQLiteRepository) DropSecure() error {
	if err := r.deleteAll(r.Cfg.GetTableMain()); err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := r.deleteAll(r.Cfg.GetTableDeleted()); err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := r.resetSQLiteSequence(r.Cfg.GetTableMain()); err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := r.resetSQLiteSequence(r.Cfg.GetTableDeleted()); err != nil {
		return fmt.Errorf("%w", err)
	}
	if err := r.vacuum(); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// Info returns the repository info
func (r *SQLiteRepository) Info() string {
	var main, deleted, header string
	main = strconv.Itoa(r.GetMaxID(r.Cfg.GetTableMain()))
	deleted = strconv.Itoa(r.GetMaxID(r.Cfg.GetTableDeleted()))
	header = format.Color(r.Cfg.GetName()).Yellow().Bold().String()
	return format.HeaderWithSection(header, []string{
		format.BulletLine("records:", main),
		format.BulletLine("deleted:", deleted),
		format.BulletLine("path:", r.Cfg.GetHome()),
	})
}
