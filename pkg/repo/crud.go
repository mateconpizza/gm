package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/haaag/gm/pkg/bookmark"
)

type Record = bookmark.Bookmark

var TableMainSchema = `
  CREATE TABLE IF NOT EXISTS %s (
      id          INTEGER PRIMARY KEY AUTOINCREMENT,
      url         TEXT    NOT NULL UNIQUE,
      title       TEXT    DEFAULT "",
      tags        TEXT    DEFAULT ",",
      desc        TEXT    DEFAULT "",
      created_at  TIMESTAMP
  )
`

// Init initialize database
func (r *SQLiteRepository) Init() error {
	var main, deleted string

	main = r.Cfg.GetTableMain()
	if err := r.TableCreate(main, TableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", main, err)
	}

	deleted = r.Cfg.GetTableDeleted()
	if err := r.TableCreate(deleted, TableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", deleted, err)
	}
	return nil
}

// Insert creates a new record
func (r *SQLiteRepository) Insert(tableName string, b *Record) (*Record, error) {
	if err := bookmark.Validate(b); err != nil {
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

	log.Printf("inserted record: %s (table: %s)\n", b.URL, tableName)

	return b, nil
}

// insertBulk creates multiple records
func (r *SQLiteRepository) insertBulk(tableName string, bs *[]Record) error {
	log.Printf("inserting %d records into table: %s", len(*bs), tableName)

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
		_, err = stmt.Exec(b.GetURL(), b.GetTitle(), b.GetTags(), b.GetDesc(), b.GetCreatedAt())
		if err != nil {
			err = tx.Rollback()
			return fmt.Errorf("%w: getting the result in insert bulk", err)
		}
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("error closing rows: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: closing stmt in insert bulk", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: committing in insert bulk", err)
	}

	return nil
}

// Update updates an existing record
func (r *SQLiteRepository) Update(tableName string, b *Record) (*Record, error) {
	if !r.RecordExists(tableName, "id", strconv.Itoa(b.GetID())) {
		return b, fmt.Errorf("%w: in updating '%s'", ErrRecordNotExists, b.GetURL())
	}

	sqlQuery := fmt.Sprintf(
		"UPDATE %s SET url = ?, title = ?, tags = ?, desc = ?, created_at = ? WHERE id = ?",
		tableName,
	)

	_, err := r.DB.Exec(sqlQuery, b.GetURL(), b.GetTitle(), b.GetTags(), b.GetDesc(), b.GetCreatedAt(), b.GetID())
	if err != nil {
		return b, fmt.Errorf("%w: %w", ErrRecordUpdate, err)
	}

	log.Printf("updated record %s (table: %s)\n", b.GetURL(), tableName)

	return b, nil
}

// updateBulk updates multiple records
func (r *SQLiteRepository) updateBulk(tableName string, bs *[]Record) error {
	if len(*bs) == 0 {
		return nil
	}

	for _, b := range *bs {
		if _, err := r.GetByID(tableName, b.GetID()); err != nil {
			return fmt.Errorf("%w: in updating '%d'", ErrRecordNotExists, b.GetID())
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
		_, err := stmt.Exec(b.GetURL(), b.GetTitle(), b.GetTags(), b.GetDesc(), b.GetCreatedAt(), b.GetID())
		if err != nil {
			return fmt.Errorf("error updating record %s: %w", b.GetURL(), err)
		}

		log.Printf("updated record %s (table: %s)\n", b.GetURL(), tableName)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %w", err)
	}

	return nil
}

// delete deletes a record
func (r *SQLiteRepository) delete(tableName string, b *Record) error {
	log.Printf("deleting record %s (table: %s)\n", b.GetURL(), tableName)

	if !r.RecordExists(tableName, "url", b.GetURL()) {
		return fmt.Errorf("error removing record %w: %s", ErrRecordNotExists, b.GetURL())
	}

	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), b.GetID())
	if err != nil {
		return fmt.Errorf("error removing record %w: %w", ErrRecordDelete, err)
	}

	log.Printf("deleted record %s (table: %s)\n", b.GetURL(), tableName)

	if r.GetMaxID(tableName) == 1 {
		err := r.resetSQLiteSequence(tableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// deleteAll deletes all records
func (r *SQLiteRepository) deleteAll(tableName string) error {
	log.Printf("deleting all records from table: %s", tableName)
	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDBDrop, err)
	}

	return nil
}

// DeleteBulk deletes multiple records
func (r *SQLiteRepository) DeleteBulk(tableName string, bIDs []int) error {
	if len(bIDs) == 0 {
		return ErrRecordIDNotProvided
	}

	log.Printf("deleting %d records from table: %s", len(bIDs), tableName)
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
			log.Printf("error closing statement: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: closing stmt in delete bulk", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%w: committing in delete bulk", err)
	}

	log.Printf("deleted %d records from table: %s", len(bIDs), tableName)
	if maxID == 0 {
		err := r.resetSQLiteSequence(tableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// DeleteAndReorder deletes the specified bookmarks from the database and
// reorders the remaining IDs.
//
// Inserts the deleted bookmarks into the deleted table.
func (r *SQLiteRepository) DeleteAndReorder(bs *[]Record, main, deleted string) error {
	ids := bookmark.ExtractIDs(bs)

	if len(ids) == 0 {
		return ErrRecordIDNotProvided
	}

	if err := r.DeleteBulk(main, ids); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	// add records to deleted table
	if err := r.insertBulk(deleted, bs); err != nil {
		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}

	// if the last record is deleted, we don't need to reorder
	// reset the SQLite sequence
	if r.GetMaxID(main) == 0 {
		err := r.resetSQLiteSequence(main)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		return nil
	}

	if err := r.reorderIDs(main); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	// FIX: check size on NewRepository
	// return r.checkSize(r.Conf.MaxSizeBytes)
	return nil
}

// GetAll returns all records
func (r *SQLiteRepository) GetAll(tableName string) (*[]Record, error) {
	log.Printf("getting all records from table: '%s'", tableName)
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", tableName)

	bs, err := r.getBySQL(sqlQuery)
	if err != nil {
		return nil, err
	}

	if len(*bs) == 0 {
		log.Printf("no records found in table: '%s'", tableName)
		return nil, ErrRecordNotFound
	}

	log.Printf("got %d records from table: '%s'", len(*bs), tableName)
	return bs, nil
}

// GetByID returns a record by its ID
func (r *SQLiteRepository) GetByID(tableName string, n int) (*Record, error) {
	if n > r.GetMaxID(tableName) {
		return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
	}
	var d Record
	log.Printf("getting record by ID %d (table: %s)\n", n, tableName)
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", tableName), n)

	err := row.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	log.Printf("got record by ID %d (table: %s)\n", n, tableName)
	return &d, nil
}

// GetByIDList returns a list of records by their IDs
func (r *SQLiteRepository) GetByIDList(tableName string, ids []int) (*[]Record, error) {
	if len(ids) == 0 {
		return nil, ErrRecordIDNotProvided
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

	records, err := r.getBySQL(query, args...)
	if err != nil {
		return nil, err
	}

	return records, nil
}

// GetByURL returns a record by its URL
func (r *SQLiteRepository) GetByURL(tableName, u string) (*Record, error) {
	var d Record
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE url = ?", tableName), u)

	err := row.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with url: %s", ErrRecordNotFound, u)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	return &d, nil
}

// getBySQL retrieves records from the SQLite database based on the provided SQL query.
func (r *SQLiteRepository) getBySQL(q string, args ...interface{}) (*[]Record, error) {
	rows, err := r.DB.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: on getting records by query", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on getting records by sql: %v", err)
		}
	}()

	var all []Record
	for rows.Next() {
		var d Record
		if err := rows.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		all = append(all, d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	return &all, nil
}

// GetByTags returns records by tag
func (r *SQLiteRepository) GetByTags(tableName, tag string) (*[]Record, error) {
	log.Printf("getting records by tag: %s", tag)
	query := fmt.Sprintf("SELECT * FROM %s WHERE tags LIKE ?", tableName)
	tag = "%" + tag + "%"
	return r.getBySQL(query, tag)
}

// GetByQuery returns records by query
func (r *SQLiteRepository) GetByQuery(tableName, q string) (*[]Record, error) {
	log.Printf("getting records by query: %s", q)

	sqlQuery := fmt.Sprintf(`
      SELECT 
        id, url, title, tags, desc, created_at
      FROM %s 
      WHERE 
        LOWER(id || title || url || tags || desc) LIKE LOWER(?)
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

// GetByColumn returns the data found from the given column name
func (r *SQLiteRepository) GetByColumn(tableName, column string) (*[]string, error) {
	log.Printf("getting all records from table: '%s' and column: '%s'", tableName, column)
	sqlQuery := fmt.Sprintf("SELECT %s FROM %s ORDER BY id ASC", column, tableName)
	rows, err := r.DB.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("getting records by column: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on getting records by sql: %v", err)
		}
	}()

	var allTags []string

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		allTags = append(allTags, tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	if len(allTags) == 0 {
		log.Printf("no tags found in table: '%s' and column: '%s'", tableName, column)
		return nil, fmt.Errorf("%w by table: '%s' and column: '%s'", ErrRecordNotFound, tableName, column)
	}

	log.Printf("tags found: %d by column: '%s'", len(allTags), column)
	return &allTags, nil
}

// GetMaxID retrieves the maximum ID from the specified table in the SQLite database.
func (r *SQLiteRepository) GetMaxID(tableName string) int {
	var lastIndex int

	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", tableName)

	if err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex); err != nil {
		log.Fatalf("getting maxID from table='%s', err='%v'", tableName, err)
	}

	log.Printf("maxID: %d", lastIndex)

	return lastIndex
}
