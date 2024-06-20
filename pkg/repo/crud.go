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
	"github.com/haaag/gm/pkg/slice"
)

type (
	IDs   = slice.Slice[int]
	Row   = bookmark.Bookmark
	Slice = slice.Slice[Row]
)

// Init initialize database
func (r *SQLiteRepository) Init() error {
	var main, deleted string

	main = r.Cfg.GetTableMain()
	if err := r.TableCreate(main, tableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", main, err)
	}

	deleted = r.Cfg.GetTableDeleted()
	if err := r.TableCreate(deleted, tableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", deleted, err)
	}
	return nil
}

// Insert creates a new record
func (r *SQLiteRepository) Insert(tableName string, b *Row) (*Row, error) {
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
		currentTime.Format(_defDateFormat),
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
func (r *SQLiteRepository) insertBulk(tableName string, bs *Slice) error {
	log.Printf("inserting %d records into table: %s", bs.Len(), tableName)

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

	err = bs.ForEachErr(func(b Row) error {
		_, err = stmt.Exec(b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt)
		if err != nil {
			return fmt.Errorf("%w: getting the result in insert bulk", err)
		}
		return nil
	})

	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: rollback on insert bulk", err)
	}

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
func (r *SQLiteRepository) Update(tableName string, b *Row) (*Row, error) {
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

	log.Printf("updated record %s (table: %s)\n", b.URL, tableName)

	return b, nil
}

// delete deletes a record
func (r *SQLiteRepository) delete(tableName string, b *Row) error {
	log.Printf("deleting record %s (table: %s)\n", b.URL, tableName)

	if !r.RecordExists(tableName, "url", b.URL) {
		return fmt.Errorf("error removing record %w: %s", ErrRecordNotExists, b.URL)
	}

	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), b.ID)
	if err != nil {
		return fmt.Errorf("error removing record %w: %w", ErrRecordDelete, err)
	}

	log.Printf("deleted record %s (table: %s)\n", b.URL, tableName)

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
func (r *SQLiteRepository) DeleteBulk(tableName string, ids *IDs) error {
	var n = ids.Len()
	if n == 0 {
		return ErrRecordIDNotProvided
	}

	log.Printf("deleting %d records from table: %s", n, tableName)
	maxID := r.GetMaxID(tableName)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction in bulk delete", err)
	}

	// TODO: replace placeholders loop with strings.Repeat
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id IN (%s)", tableName, strings.Repeat("?,", n-1)+"?")

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: prepared statement for use within a transaction in bulk delete", err)
	}

	args := make([]interface{}, n)
	for i, id := range *ids.GetAll() {
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

	log.Printf("deleted %d records from table: %s", n, tableName)
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
func (r *SQLiteRepository) DeleteAndReorder(bs *Slice, main, deleted string) error {
	if bs.Len() == 0 {
		return ErrRecordIDNotProvided
	}

	var ids = slice.New[int]()
	bs.ForEach(func(r Row) {
		ids.Add(&r.ID)
	})

	if ids.Len() == 0 {
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

	return nil
}

// GetAll returns all records
func (r *SQLiteRepository) GetAll(tableName string, bs *Slice) error {
	log.Printf("getting all records from table: '%s'", tableName)
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", tableName)

	if err := r.getBySQL(bs, sqlQuery); err != nil {
		return err
	}

	if bs.Len() == 0 {
		log.Printf("no records found in table: '%s'", tableName)
		return ErrRecordNotFound
	}

	log.Printf("got %d records from table: '%s'", bs.Len(), tableName)
	return nil
}

// GetByID returns a record by its ID
func (r *SQLiteRepository) GetByID(tableName string, n int) (*Row, error) {
	if n > r.GetMaxID(tableName) {
		return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
	}
	var d Row
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
func (r *SQLiteRepository) GetByIDList(tableName string, ids []int, bs *Slice) error {
	if len(ids) == 0 {
		return ErrRecordIDNotProvided
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

	return r.getBySQL(bs, query, args...)
}

// GetByURL returns a record by its URL
func (r *SQLiteRepository) GetByURL(tableName, u string) (*Row, error) {
	var d Row
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
func (r *SQLiteRepository) getBySQL(bs *Slice, q string, args ...interface{}) error {
	// FIX: replace `all` with Slice
	rows, err := r.DB.Query(q, args...)
	if err != nil {
		return fmt.Errorf("%w: on getting records by query", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on getting records by sql: %v", err)
		}
	}()

	var all []Row
	for rows.Next() {
		var d Row
		if err := rows.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt); err != nil {
			return fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		all = append(all, d)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	bs.Set(&all)
	return nil
}

// GetByTags returns records by tag
func (r *SQLiteRepository) GetByTags(tableName, tag string, bs *Slice) error {
	log.Printf("getting records by tag: %s", tag)
	query := fmt.Sprintf("SELECT * FROM %s WHERE tags LIKE ?", tableName)
	tag = "%" + tag + "%"

	return r.getBySQL(bs, query, tag)
}

// GetByQuery returns records by query
func (r *SQLiteRepository) GetByQuery(tableName, q string, bs *Slice) error {
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
	if err := r.getBySQL(bs, sqlQuery, queryValue); err != nil {
		return err
	}

	var n = bs.Len()
	if n == 0 {
		return ErrRecordNoMatch
	}

	log.Printf("got %d records by query: '%s'", n, q)
	return nil
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
