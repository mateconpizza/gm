package bookmark

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gomarks/pkg/config"

	_ "github.com/mattn/go-sqlite3"
)

var (
	// database
	ErrDBAlreadyExists      = errors.New("database already exists")
	ErrDBAlreadyInitialized = errors.New("database already initialized")
	ErrDBNotInitialized     = errors.New("database not initialized")
	ErrDBNotFound           = errors.New("database not found")
	ErrDBsNotFound          = errors.New("no database/s found")
	ErrDBResetSequence      = errors.New("resetting sqlite_sequence")
	ErrDBSpecify            = errors.New("specify a database name")
	ErrDBDrop               = errors.New("dropping database")
	ErrSQLQuery             = errors.New("executing query")
	ErrDBEmpty              = errors.New("database is empty")
	// records
	ErrRecordDelete       = errors.New("error delete record")
	ErrRecordDuplicate    = errors.New("record already exists")
	ErrRecordInsert       = errors.New("inserting record")
	ErrRecordNotExists    = errors.New("row not exists")
	ErrRecordScan         = errors.New("scan record")
	ErrRecordUpdate       = errors.New("update failed")
	ErrRecordNotFound     = errors.New("no bookmarks found")
	ErrNoRecordIDProvided = errors.New("no id provided")
	ErrActionAborted      = errors.New("action aborted")
	ErrNoQueryProvided    = errors.New("no id or query provided")
)

// SQLiteRepository implements the Repository interface
type SQLiteRepository struct {
	DB *sql.DB
}

// NewSQLiteRepository returns a new SQLiteRepository
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		DB: db,
	}
}

// TODO:
// [ ] add `last_used` column
// [ ] add `last_checked` column
//
/* var TableMainSchema = `
   CREATE TABLE IF NOT EXISTS %s (
       id           INTEGER PRIMARY KEY AUTOINCREMENT,
       url          TEXT    NOT NULL UNIQUE,
       title        TEXT    DEFAULT "",
       tags         TEXT    DEFAULT ",",
       desc         TEXT    DEFAULT "",
       created_at   TIMESTAMP,
       last_used    TIMESTAMP,
       last_checked TIMESTAMP,
       status       BOOL
   )
` */

var tableMainSchema = `
  CREATE TABLE IF NOT EXISTS %s (
      id          INTEGER PRIMARY KEY AUTOINCREMENT,
      url         TEXT    NOT NULL UNIQUE,
      title       TEXT    DEFAULT "",
      tags        TEXT    DEFAULT ",",
      desc        TEXT    DEFAULT "",
      created_at  TIMESTAMP
  )
`

// NewRepository returns a new SQLiteRepository
func NewRepository(name string) (*SQLiteRepository, error) {
	config.LoadRepoPath(name)

	db, err := sql.Open("sqlite3", config.DB.Path)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	// FIX: find better way to update DB.Name
	// SELECT * FROM pragma_database_list; maybe?
	config.DB.Name = name
	r := NewSQLiteRepository(db)
	if exists, _ := r.tableExists(config.DB.Table.Main); !exists {
		return r, fmt.Errorf("%w", ErrDBNotFound)
	}

	if _, err := r.size(); err != nil {
		return r, fmt.Errorf("%w", err)
	}

	return r, nil
}

// Init initializes the database
func (r *SQLiteRepository) Init() error {
	if err := r.tableCreate(config.DB.Table.Main, tableMainSchema); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := r.tableCreate(config.DB.Table.Deleted, tableMainSchema); err != nil {
		return fmt.Errorf("%w", err)
	}

	initialBookmark := NewBookmark(
		config.App.Data.URL,
		config.App.Data.Title,
		config.App.Data.Tags,
		config.App.Data.Desc,
	)

	if _, err := r.Insert(config.DB.Table.Main, initialBookmark); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// Insert creates a new bookmark
func (r *SQLiteRepository) Insert(tableName string, b *Bookmark) (*Bookmark, error) {
	if err := Validate(b); err != nil {
		return nil, fmt.Errorf("abort: %w", err)
	}

	if r.RecordExists(tableName, "url", b.URL) {
		return nil, fmt.Errorf(
			"%w: '%s' in table '%s'",
			ErrRecordDuplicate,
			b.URL,
			tableName,
		)
	}

	currentTime := time.Now()
	sqlQuery := fmt.Sprintf(
		`INSERT INTO %s(
      url, title, tags, desc, created_at)
      VALUES(?, ?, ?, ?, ?)`, tableName)
	result, err := r.DB.Exec(
		sqlQuery,
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		currentTime.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: '%s'", ErrRecordInsert, b.URL)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRecordInsert, err)
	}

	b.ID = int(id)

	log.Printf("Inserted bookmark: %s (table: %s)\n", b.URL, tableName)

	return b, nil
}

// InsertBulk creates multiple bookmarks
func (r *SQLiteRepository) InsertBulk(tableName string, bs *[]Bookmark) error {
	log.Printf("Inserting %d bookmarks into table: %s", len(*bs), tableName)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction in bulk insert", err)
	}

	sqlQuery := fmt.Sprintf(
		"INSERT OR IGNORE INTO %s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		tableName,
	)

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: prepared statement for use within a transaction in bulk insert", err)
	}

	for _, b := range *bs {
		_, err = stmt.Exec(b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt)
		if err != nil {
			err = tx.Rollback()
			return fmt.Errorf("%w: getting the result in insert bulk", err)
		}
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: closing stmt in insert bulk", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: committing in insert bulk", err)
	}

	log.Printf("Inserted %d bookmarks into table: %s", len(*bs), tableName)

	return nil
}

// Update updates an existing bookmark
func (r *SQLiteRepository) Update(tableName string, b *Bookmark) (*Bookmark, error) {
	if !r.RecordExists(tableName, "id", strconv.Itoa(b.ID)) {
		return b, fmt.Errorf("%w: in updating '%s'", ErrRecordNotExists, b.URL)
	}

	sqlQuery := fmt.Sprintf(
		"UPDATE %s SET url = ?, title = ?, tags = ?, desc = ?, created_at = ? WHERE id = ?",
		tableName,
	)

	_, err := r.DB.Exec(sqlQuery, b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt, b.ID)
	if err != nil {
		return b, fmt.Errorf("%w: %w", ErrRecordUpdate, err)
	}

	log.Printf("Updated bookmark %s (table: %s)\n", b.URL, tableName)

	return b, nil
}

// UpdateBulk updates multiple bookmarks
func (r *SQLiteRepository) updateBulk(tableName string, bs *[]Bookmark) error {
	if len(*bs) == 0 {
		return nil
	}

	for _, b := range *bs {
		if _, err := r.GetByID(tableName, b.ID); err != nil {
			return fmt.Errorf("%w: in updating '%d'", ErrRecordNotExists, b.ID)
		}
	}

	sqlQuery := fmt.Sprintf(
		"UPDATE %s SET url = ?, title = ?, tags = ?, desc = ?, created_at = ? WHERE id = ?",
		tableName,
	)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("error starting transaction: %w", err)
	}
	defer func() {
		if err = tx.Rollback(); err != nil {
			log.Printf("error rolling back transaction: %v", err)
		}
	}()

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		return fmt.Errorf("error preparing SQL statement: %w", err)
	}
	defer func() {
		if cerr := stmt.Close(); cerr != nil {
			log.Printf("error closing statement: %v", cerr)
		}
	}()

	for _, b := range *bs {
		_, err := stmt.Exec(b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt, b.ID)
		if err != nil {
			return fmt.Errorf("error updating bookmark %s: %w", b.URL, err)
		}

		log.Printf("Updated bookmark %s (table: %s)\n", b.URL, tableName)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// Delete deletes a bookmark
func (r *SQLiteRepository) Delete(tableName string, b *Bookmark) error {
	log.Printf("Deleting bookmark %s (table: %s)\n", b.URL, tableName)

	if !r.RecordExists(tableName, "url", b.URL) {
		return fmt.Errorf("error removing bookmark %w: %s", ErrRecordNotExists, b.URL)
	}

	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), b.ID)
	if err != nil {
		return fmt.Errorf("error removing bookmark %w: %w", ErrRecordDelete, err)
	}

	log.Printf("Deleted bookmark %s (table: %s)\n", b.URL, tableName)

	if r.GetMaxID(tableName) == 1 {
		err := r.ResetSQLiteSequence(tableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// DeleteAll deletes all bookmarks
func (r *SQLiteRepository) DeleteAll(tableName string) error {
	log.Printf("Deleting all bookmarks from table: %s", tableName)
	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDBDrop, err)
	}

	return nil
}

// DeleteBulk deletes multiple bookmarks
func (r *SQLiteRepository) DeleteBulk(tableName string, bIDs []int) error {
	if len(bIDs) == 0 {
		return ErrBookmarkNotSelected
	}

	log.Printf("Deleting %d bookmarks from table: %s", len(bIDs), tableName)
	maxID := r.GetMaxID(tableName)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction in bulk delete", err)
	}

	// TODO: replace placeholders loop with strings.Repeat
	// query := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", tableName, strings.Repeat("?,", len(bIDs)-1)+"?")
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id IN (", tableName)
	placeholders := make([]string, len(bIDs))
	for i := 0; i < len(bIDs); i++ {
		placeholders[i] = "?"
	}
	sqlQuery += strings.Join(placeholders, ",") + ")"

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: prepared statement for use within a transaction in bulk delete", err)
	}

	args := make([]interface{}, len(bIDs))
	for i, id := range bIDs {
		args[i] = id
	}

	_, err = stmt.Exec(args...)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: getting the result in delete bulk", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("Error closing statement: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: closing stmt in delete bulk", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: committing in delete bulk", err)
	}

	log.Printf("Deleted %d bookmarks from table: %s", len(bIDs), tableName)

	if maxID == 1 {
		err := r.ResetSQLiteSequence(tableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// GetByID returns a bookmark by its ID
func (r *SQLiteRepository) GetByID(tableName string, n int) (*Bookmark, error) {
	log.Printf("getting bookmark by ID %d (table: %s)\n", n, tableName)
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tableName), n)

	var b Bookmark

	err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	log.Printf("got bookmark by ID %d (table: %s)\n", n, tableName)
	return &b, nil
}

// GetByIDList returns a list of bookmarks by their IDs
func (r *SQLiteRepository) GetByIDList(tableName string, ids []int) (*[]Bookmark, error) {
	if len(ids) == 0 {
		return nil, ErrNoRecordIDProvided
	}

	placeholders := make([]string, len(ids))
	for i := 0; i < len(ids); i++ {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE ID IN (%s);", tableName, strings.Repeat("?,", len(ids)-1)+"?",
	)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	bookmarks, err := r.getBySQL(query, args...)
	if err != nil {
		return nil, err
	}

	if len(*bookmarks) != len(ids) {
		logItemsNotFound(bookmarks, ids)
	}

	return bookmarks, nil
}

// GetByURL returns a bookmark by its URL
func (r *SQLiteRepository) GetByURL(tableName, u string) (*Bookmark, error) {
	var b Bookmark
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE url = ?", tableName), u)

	err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with url: %s", ErrRecordNotFound, u)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	return &b, nil
}

// getBySQL retrieves bookmarks from the SQLite database based on the provided SQL query.
func (r *SQLiteRepository) getBySQL(q string, args ...interface{}) (*[]Bookmark, error) {
	rows, err := r.DB.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: on getting records by query", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows on getting records by sql: %v", err)
		}
	}()

	var all []Bookmark

	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		all = append(all, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	return &all, nil
}

// GetAll returns all bookmarks
func (r *SQLiteRepository) GetAll(tableName string) (*[]Bookmark, error) {
	log.Printf("getting all records from table: '%s'", tableName)
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", tableName)

	bs, err := r.getBySQL(sqlQuery)
	if err != nil {
		return nil, err
	}

	if len(*bs) == 0 {
		log.Printf("No records found in table: '%s'", tableName)

		return nil, ErrRecordNotFound
	}

	log.Printf("got %d records from table: '%s'", len(*bs), tableName)
	return bs, nil
}

// GetByTags returns bookmarks by tag
func (r *SQLiteRepository) GetByTags(tableName, tag string) (*[]Bookmark, error) {
	log.Printf("getting records by tag: %s", tag)
	query := fmt.Sprintf("SELECT * FROM %s WHERE tags LIKE ?", tableName)
	tag = "%" + tag + "%"
	return r.getBySQL(query, tag)
}

// GetByQuery returns bookmarks by query
func (r *SQLiteRepository) GetByQuery(tableName, q string) (*[]Bookmark, error) {
	log.Printf("getting records by query: %s", q)

	sqlQuery := fmt.Sprintf(`
      SELECT 
        id, url, title, tags, desc, created_at
      FROM %s 
      WHERE 
        id || title || url || tags || desc LIKE ?
      ORDER BY id ASC
    `, tableName)

	queryValue := "%" + q + "%"
	bs, err := r.getBySQL(sqlQuery, queryValue)
	if err != nil {
		return nil, err
	}

	if len(*bs) == 0 {
		return nil, ErrRecordNotFound
	}

	log.Printf("got %d records by query: '%s'", len(*bs), q)
	return bs, err
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

// GetMaxID retrieves the maximum ID from the specified table in the SQLite database.
func (r *SQLiteRepository) GetMaxID(tableName string) int {
	var lastIndex int

	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", tableName)

	if err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex); err != nil {
		log.Fatal(err)
	}

	log.Printf("Max ID: %d", lastIndex)

	return lastIndex
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
	log.Printf("Renaming table %s to %s", tempTable, mainTable)

	_, err := r.DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tempTable, mainTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from '%s' to '%s'", err, tempTable, mainTable)
	}

	log.Printf("Renamed table %s to %s\n", tempTable, mainTable)

	return nil
}

// tableCreate creates a new table with the specified name in the SQLite database.
func (r *SQLiteRepository) tableCreate(name, schema string) error {
	log.Printf("Creating table: %s", name)
	tableSchema := fmt.Sprintf(schema, name)

	_, err := r.DB.Exec(tableSchema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	log.Printf("created table: %s\n", name)

	return nil
}

// tableDrop drops the specified table from the SQLite database.
func (r *SQLiteRepository) tableDrop(t string) error {
	log.Printf("Dropping table: %s", t)

	_, err := r.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table '%s'", err, t)
	}

	log.Printf("Dropped table: %s\n", t)

	return nil
}

// ResetSQLiteSequence resets the SQLite sequence for the given table.
func (r *SQLiteRepository) ResetSQLiteSequence(t string) error {
	if _, err := r.DB.Exec("DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
		return fmt.Errorf("resetting sqlite sequence: %w", err)
	}

	return nil
}

// ReorderIDs reorders the IDs in the specified table.
func (r *SQLiteRepository) ReorderIDs(tableName string) error {
	// FIX: Everytime we re-order IDs, the db's size gets bigger
	// It's a bad implementation? (but it works)
	// Maybe use 'VACUUM' command?
	bs, err := r.GetAll(tableName)
	if err != nil {
		return err
	}

	if len(*bs) == 0 {
		return nil
	}

	log.Printf("Reordering IDs in table: %s", tableName)

	tempTable := fmt.Sprintf("temp_%s", tableName)
	if err := r.tableCreate(tempTable, tableMainSchema); err != nil {
		return err
	}

	if err := r.InsertBulk(tempTable, bs); err != nil {
		return err
	}

	if err := r.tableDrop(tableName); err != nil {
		return err
	}

	return r.tableRename(tempTable, tableName)
}

// DeleteAndReorder deletes the specified bookmarks from the database and
// reorders the remaining IDs.
func (r *SQLiteRepository) DeleteAndReorder(bs *[]Bookmark) error {
	if err := r.DeleteBulk(config.DB.Table.Main, ExtractIDs(bs)); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	// add records to deleted table
	if err := r.InsertBulk(config.DB.Table.Deleted, bs); err != nil {
		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}

	// if the last record is deleted, we don't need to reorder
	maxID := r.GetMaxID(config.DB.Table.Main)
	if maxID == 0 {
		return nil
	}

	if err := r.ReorderIDs(config.DB.Table.Main); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	size, err := r.size()
	if err != nil {
		return fmt.Errorf("size: %w", err)
	}

	if size > config.DB.MaxSizeBytes {
		return r.Vacuum()
	}

	return nil
}

// Vacuum rebuilds the database file, repacking it into a minimal amount of disk space.
func (r *SQLiteRepository) Vacuum() error {
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
func (r *SQLiteRepository) IsEmpty() bool {
	if r.GetMaxID(config.DB.Table.Main) > 0 || r.GetMaxID(config.DB.Table.Deleted) > 0 {
		return false
	}
	return true
}
