package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/slice"
)

type (
	Row   = bookmark.Bookmark
	Slice = slice.Slice[Row]
	Table string
)

// Init initializes a new database and creates the required tables.
func (r *SQLiteRepository) Init() error {
	if err := r.TableCreate(r.Cfg.TableMain, tableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", r.Cfg.TableMain, err)
	}

	if err := r.TableCreate(r.Cfg.TableDeleted, tableMainSchema); err != nil {
		return fmt.Errorf("creating '%s' table: %w", r.Cfg.TableDeleted, err)
	}

	return nil
}

// Insert creates a new record in the given table.
func (r *SQLiteRepository) Insert(t Table, b *Row) (*Row, error) {
	if err := bookmark.Validate(b); err != nil {
		return nil, fmt.Errorf("abort: %w", err)
	}

	if r.HasRecord(t, "url", b.URL) {
		return nil, fmt.Errorf(
			"%w: '%s' in table '%s'",
			ErrRecordDuplicate,
			b.URL,
			t,
		)
	}

	ct := time.Now()
	query := fmt.Sprintf(
		`INSERT INTO %s(
      url, title, tags, desc, created_at)
      VALUES(?, ?, ?, ?, ?)`, t)

	result, err := r.DB.Exec(
		query,
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		ct.Format(config.DB.DateFormat),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: '%s'", ErrRecordInsert, b.URL)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrRecordInsert, err)
	}

	b.ID = int(id)

	log.Printf("inserted record: %s (table: %s)\n", b.URL, t)

	return b, nil
}

// insertBulk creates multiple records in the give table.
func (r *SQLiteRepository) insertBulk(t Table, bs *Slice) error {
	log.Printf("inserting %d records into table: %s", bs.Len(), t)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction in bulk insert", err)
	}

	sqlQuery := fmt.Sprintf(
		"INSERT OR IGNORE INTO %s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		t,
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

// Update updates an existing record in the give table.
func (r *SQLiteRepository) Update(t Table, b *Row) (*Row, error) {
	if !r.HasRecord(t, "id", strconv.Itoa(b.ID)) {
		return b, fmt.Errorf("%w: in updating '%s'", ErrRecordNotExists, b.URL)
	}

	sqlQuery := fmt.Sprintf(
		"UPDATE %s SET url = ?, title = ?, tags = ?, desc = ?, created_at = ? WHERE id = ?",
		t,
	)

	_, err := r.DB.Exec(sqlQuery, b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt, b.ID)
	if err != nil {
		return b, fmt.Errorf("%w: %w", ErrRecordUpdate, err)
	}

	log.Printf("updated record %s (table: %s)\n", b.URL, t)

	return b, nil
}

// delete deletes a record in the give table.
func (r *SQLiteRepository) delete(t Table, b *Row) error {
	log.Printf("deleting record %s (table: %s)\n", b.URL, t)

	if !r.HasRecord(t, "url", b.URL) {
		return fmt.Errorf("error removing record %w: %s", ErrRecordNotExists, b.URL)
	}

	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", t), b.ID)
	if err != nil {
		return fmt.Errorf("error removing record %w: %w", ErrRecordDelete, err)
	}

	log.Printf("deleted record %s (table: %s)\n", b.URL, t)

	if r.maxID(t) == 1 {
		err := r.resetSQLiteSequence(t)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	return nil
}

// deleteAll deletes all records in the give table.
func (r *SQLiteRepository) deleteAll(t Table) error {
	log.Printf("deleting all records from table: %s", t)
	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s", t))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDBDrop, err)
	}

	return nil
}

// DeleteBulk deletes multiple records in the give table.
func (r *SQLiteRepository) DeleteBulk(t Table, ids *slice.Slice[int]) error {
	n := ids.Len()
	if n == 0 {
		return ErrRecordIDNotProvided
	}

	log.Printf("deleting %d records from table: %s", n, t)
	maxID := r.maxID(t)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("%w: begin starts a transaction in bulk delete", err)
	}

	sqlQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE id IN (%s)",
		t,
		strings.Repeat("?,", n-1)+"?",
	)

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("%w: prepared statement for use within a transaction in bulk delete", err)
	}

	args := make([]interface{}, n)
	for i, id := range *ids.Items() {
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

	log.Printf("deleted %d records from table: %s", n, t)
	if maxID == 0 {
		err := r.resetSQLiteSequence(t)
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
func (r *SQLiteRepository) DeleteAndReorder(bs *Slice, main, deleted Table) error {
	if bs.Len() == 0 {
		return ErrRecordIDNotProvided
	}

	ids := slice.New[int]()
	bs.ForEach(func(r Row) {
		ids.Append(&r.ID)
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
	if r.maxID(main) == 0 {
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

// Records returns all records in the give table.
func (r *SQLiteRepository) Records(t Table, bs *Slice) error {
	log.Printf("getting all records from table: '%s'", t)
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", t)

	if err := r.bySQL(bs, sqlQuery); err != nil {
		return err
	}

	if bs.Len() == 0 {
		log.Printf("no records found in table: '%s'", t)
		return ErrRecordNotFound
	}

	log.Printf("got %d records from table: '%s'", bs.Len(), t)

	return nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLiteRepository) ByID(t Table, n int) (*Row, error) {
	if n > r.maxID(t) {
		return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
	}

	var d Row
	log.Printf("getting record by ID %d (table: %s)\n", n, t)
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", t), n)

	err := row.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	log.Printf("got record by ID %d (table: %s)\n", n, t)

	return &d, nil
}

// ByIDList returns a list of records by their IDs in the give table.
func (r *SQLiteRepository) ByIDList(t Table, ids []int, bs *Slice) error {
	if len(ids) == 0 {
		return ErrRecordIDNotProvided
	}

	placeholders := make([]string, len(ids))
	for i := 0; i < len(ids); i++ {
		placeholders[i] = "?"
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE ID IN (%s);", t, strings.Repeat("?,", len(ids)-1)+"?",
	)

	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	return r.bySQL(bs, query, args...)
}

// ByURL returns a record by its URL in the give table.
func (r *SQLiteRepository) ByURL(t Table, u string) (*Row, error) {
	var d Row
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE url = ?", t), u)

	err := row.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with url: %s", ErrRecordNotFound, u)
		}

		return nil, fmt.Errorf("%w: %w", ErrRecordScan, err)
	}

	return &d, nil
}

// bySQL retrieves records from the SQLite database based on the provided SQL query.
func (r *SQLiteRepository) bySQL(bs *Slice, q string, args ...interface{}) error {
	rows, err := r.DB.Query(q, args...)
	if err != nil {
		return fmt.Errorf("%w: on getting records by query", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on getting records by sql: %v", err)
		}
	}()

	for rows.Next() {
		var d Row
		if err := rows.Scan(&d.ID, &d.URL, &d.Title, &d.Tags, &d.Desc, &d.CreatedAt); err != nil {
			return fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		bs.Append(&d)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	return nil
}

// ByTag returns records by tag in the give table.
func (r *SQLiteRepository) ByTag(t Table, tag string, bs *Slice) error {
	// FIX: make it case insensitive
	log.Printf("getting records by tag: %s", tag)
	query := fmt.Sprintf("SELECT * FROM %s WHERE tags LIKE ?", t)
	tag = "%" + tag + "%"

	return r.bySQL(bs, query, tag)
}

// ByQuery returns records by query in the give table.
func (r *SQLiteRepository) ByQuery(t Table, q string, bs *Slice) error {
	log.Printf("getting records by query: '%s'", q)

	sqlQuery := fmt.Sprintf(`
      SELECT 
        id, url, title, tags, desc, created_at
      FROM %s 
      WHERE 
        LOWER(id || title || url || tags || desc) LIKE LOWER(?)
      ORDER BY id ASC
    `, t)

	queryValue := "%" + q + "%"
	if err := r.bySQL(bs, sqlQuery, queryValue); err != nil {
		return err
	}

	n := bs.Len()
	if n == 0 {
		return ErrRecordNoMatch
	}

	log.Printf("got %d records by query: '%s'", n, q)

	return nil
}

// ByColumn returns the data found from the given column name.
func (r *SQLiteRepository) ByColumn(t Table, c string) (*slice.Slice[string], error) {
	log.Printf("getting all records from table: '%s' and column: '%s'", t, c)
	sqlQuery := fmt.Sprintf("SELECT %s FROM %s ORDER BY id ASC", c, t)
	rows, err := r.DB.Query(sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("getting records by column: %w", err)
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("error closing rows on getting records by sql: %v", err)
		}
	}()

	data := slice.New[string]()
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, fmt.Errorf("%w: '%w'", ErrRecordScan, err)
		}
		data.Append(&tag)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	n := data.Len()
	if n == 0 {
		log.Printf("no tags found in table: '%s' and column: '%s'", t, c)
		return nil, fmt.Errorf(
			"%w by table: '%s' and column: '%s'",
			ErrRecordNotFound,
			t,
			c,
		)
	}

	log.Printf("tags found: %d by column: '%s'", n, c)

	return data, nil
}

// maxID retrieves the maximum ID from the specified table in the SQLite database.
func (r *SQLiteRepository) maxID(t Table) int {
	var lastIndex int

	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", t)

	if err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex); err != nil {
		log.Fatalf(
			"getting maxID from table='%s' in database='%s', err='%v'",
			t,
			r.Cfg.Name,
			err,
		)
	}

	log.Printf("maxID: %d", lastIndex)

	return lastIndex
}
