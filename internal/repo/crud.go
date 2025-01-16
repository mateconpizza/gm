package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/slice"
)

type (
	Row   = bookmark.Bookmark
	Slice = slice.Slice[Row]
	Table string
)

// InsertInto creates a new record in the given tables.
func (r *SQLiteRepository) InsertInto(tmain, trecords, ttags Table, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}

	if r.HasRecord(tmain, "url", b.URL) {
		return ErrRecordDuplicate
	}

	tx, err := r.DB.Begin()
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("begin starts a transaction: %w", err)
	}

	if err := insertRecord(tx, tmain, b); err != nil {
		return err
	}

	if err := r.associateTags(tx, trecords, ttags, b); err != nil {
		return fmt.Errorf("%w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCommit, err)
	}

	log.Printf("inserted record: %s (table: %s)\n", b.URL, tmain)

	return nil
}

// insertRecord inserts a new record into the table.
func insertRecord(tx *sql.Tx, t Table, b *Row) error {
	query := fmt.Sprintf(`
    INSERT OR IGNORE INTO %s
    (url, title, desc) VALUES (?, ?, ?)`, t)

	result, err := tx.Exec(query, b.URL, b.Title, b.Desc)
	if err != nil {
		return fmt.Errorf("%w: '%s'", ErrRecordInsert, b.URL)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRecordInsert, err)
	}

	b.ID = int(id)

	return nil
}

// insertBulkNNew creates multiple records in the given tables.
func (r *SQLiteRepository) insertBulk(tmain, trecords, ttags Table, bs *Slice) error {
	log.Printf("inserting %d records into table: %s", bs.Len(), tmain)

	inserter := func(b Row) error {
		return r.InsertInto(tmain, trecords, ttags, &b)
	}

	if err := bs.ForEachErr(inserter); err != nil {
		return fmt.Errorf("insertBulk %w: inserting in bulk", err)
	}

	return nil
}

// Update updates an existing record in the given table.
func (r *SQLiteRepository) Update(t Table, b *Row) (*Row, error) {
	if !r.HasRecord(t, "url", b.URL) || !r.HasRecord(t, "id", b.ID) {
		return b, fmt.Errorf("Update: %w: in updating '%s'", ErrRecordNotExists, b.URL)
	}

	tx, err := r.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("Update: %w: begin starts a transaction", err)
	}

	sqlQuery := fmt.Sprintf("UPDATE %s SET url = ?, title = ?, desc = ? WHERE id = ?", t)
	_, err = r.DB.Exec(sqlQuery, b.URL, b.Title, b.Desc, b.ID)
	if err != nil {
		return b, fmt.Errorf("Update: %w: %w", ErrRecordUpdate, err)
	}

	if err := r.updateTags(tx, b); err != nil {
		errRollBack := tx.Rollback()
		return nil, fmt.Errorf("Update: %w: %w: rollback %w", ErrRecordUpdate, err, errRollBack)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("Update: %w: committing", err)
	}

	log.Printf("Update: updated record %s (table: %s)\n", b.URL, t)

	return b, nil
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

// deleteBulk deletes multiple records in the give table.
func (r *SQLiteRepository) deleteBulk(t Table, ids *slice.Slice[int]) error {
	n := ids.Len()
	if n == 0 {
		return ErrRecordIDNotProvided
	}

	log.Printf("deleting %d records from table: %s", n, t)
	maxID := r.maxID(t)

	tx, err := r.DB.Begin()
	if err != nil {
		return fmt.Errorf("DeleteBulk: %w: begin starts a transaction", err)
	}

	sqlQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE id IN (%s)",
		t,
		strings.Repeat("?,", n-1)+"?",
	)

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("DeleteBulk: %w: prepared statement", err)
	}

	args := make([]interface{}, n)
	for i, id := range *ids.Items() {
		args[i] = id
	}

	_, err = stmt.Exec(args...)
	if err != nil {
		err = tx.Rollback()
		return fmt.Errorf("DeleteBulk: %w: getting the result", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("error closing statement: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return fmt.Errorf("DeleteBulk: %w: closing stmt", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteBulk: %w: committing", err)
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
	if bs.Empty() {
		return ErrRecordIDNotProvided
	}

	ids := slice.New[int]()
	bs.ForEach(func(b Row) {
		ids.Append(&b.ID)
	})

	if ids.Empty() {
		return ErrRecordIDNotProvided
	}

	if err := r.deleteBulk(main, ids); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}

	// add records to deleted table
	if err := r.insertBulk(deleted, r.Cfg.Tables.RecordsTagsDeleted, r.Cfg.Tables.Tags, bs); err != nil {
		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}

	// if the last record is deleted, we don't need to reorder
	// reset the SQLite sequence
	if r.maxID(main) == 0 {
		err := r.resetSQLiteSequence(main, deleted)
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

	if bs.Empty() {
		log.Printf("no records found in table: '%s'", t)
		return ErrRecordNotFound
	}

	log.Printf("got %d records from table: '%s'", bs.Len(), t)

	return nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLiteRepository) ByID(t Table, n int) (*Row, error) {
	if n > r.maxID(t) {
		return nil, fmt.Errorf("%w with id: %d, maxID: %d", ErrRecordNotFound, n, r.maxID(t))
	}

	var b Row
	log.Printf("getting record by ID=%d (table: %s)\n", n, t)
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", t), n)

	err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Desc, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, n)
		}

		return nil, fmt.Errorf("ByID: %w: %w", ErrRecordScan, err)
	}

	if err := r.loadRecordTags(&b); err != nil {
		return nil, err
	}

	log.Printf("got record by ID %d (table: %s)\n", n, t)

	return &b, nil
}

// ByIDList returns a list of records by their IDs in the give table.
func (r *SQLiteRepository) ByIDList(t Table, ids []int, bs *Slice) error {
	if len(ids) == 0 {
		return ErrRecordIDNotProvided
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
func (r *SQLiteRepository) ByURL(t Table, bURL string) (*Row, error) {
	var b Row
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE url = ?", t), bURL)
	err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Desc, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with url: %s", ErrRecordNotFound, bURL)
		}

		return nil, fmt.Errorf("ByURL %w: %w", ErrRecordScan, err)
	}

	if err := r.loadRecordTags(&b); err != nil {
		return nil, err
	}

	return &b, nil
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
		if err := rows.Scan(&d.ID, &d.URL, &d.Title, &d.Desc, &d.CreatedAt); err != nil {
			return fmt.Errorf("bySQL: %w: '%w'", ErrRecordScan, err)
		}
		bs.Append(&d)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("%w: closing rows on getting records by query", err)
	}

	if err := r.populateTags(bs); err != nil {
		return nil
	}

	bs.Sort(func(a, b Row) bool {
		return a.ID < b.ID
	})

	return nil
}

// ByTag returns records filtered by tag.
func (r *SQLiteRepository) ByTag(tag string, bs *Slice) error {
	q := fmt.Sprintf(`
        SELECT b.id, b.url, b.title, b.desc, b.created_at
        FROM %s b
        JOIN %s bt ON b.url = bt.bookmark_url
        JOIN tags t ON bt.tag_id = t.id
        WHERE t.name LIKE ?`,
		r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags)

	return r.bySQL(bs, q, "%"+tag+"%")
}

// ByQuery returns records by query in the give table.
func (r *SQLiteRepository) ByQuery(t Table, q string, bs *Slice) error {
	log.Printf("getting records by query: '%s'", q)

	sqlQuery := fmt.Sprintf(`
      SELECT DISTINCT
        b.id, b.url, b.title, b.desc, b.created_at
      FROM %s b
      LEFT JOIN %s bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
      WHERE
        LOWER(b.id || b.title || b.url || b.desc) LIKE LOWER(?) OR
        LOWER(t.name) LIKE LOWER(?)
      ORDER BY b.id ASC
    `, t, r.Cfg.Tables.RecordsTags)

	queryValue := "%" + q + "%"
	if err := r.bySQL(bs, sqlQuery, queryValue, queryValue); err != nil {
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
			return nil, fmt.Errorf("ByColumn: %w: '%w'", ErrRecordScan, err)
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

// HasRecord checks if a record exists in the specified table and column.
func (r *SQLiteRepository) HasRecord(t Table, column, target any) bool {
	var recordCount int

	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s=?", t, column)

	if err := r.DB.QueryRow(sqlQuery, target).Scan(&recordCount); err != nil {
		log.Fatal(err)
	}

	return recordCount > 0
}

// maxID retrieves the maximum ID from the specified table in the SQLite
// database.
func (r *SQLiteRepository) maxID(t Table) int {
	var lastIndex int
	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", t)

	if err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex); err != nil {
		return 0
	}

	return lastIndex
}

// reorderIDs reorders the IDs in the specified table.
func (r *SQLiteRepository) reorderIDs(t Table) error {
	// FIX: Every time we re-order IDs, the db's size gets bigger
	// It's a bad implementation? (but it works)
	// Maybe use 'VACUUM' command? it is safe?
	bs := slice.New[Row]()
	if err := r.Records(t, bs); err != nil {
		return err
	}

	if bs.Empty() {
		return nil
	}

	log.Printf("reordering IDs in table: %s", t)
	tempTable := "temp_" + t
	if err := r.TableCreate(tempTable, tableMainSchema); err != nil {
		return err
	}

	if err := r.insertBulk(tempTable, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, bs); err != nil {
		return err
	}

	if err := r.TableDrop(t); err != nil {
		return err
	}

	return r.tableRename(tempTable, t)
}

// Restore restores record/s from deleted tabled.
func (r *SQLiteRepository) Restore(tx *sql.Tx, bs *Slice) error {
	// FIX: remove `config.DB.Tables.Main`
	tmain := Table(config.DB.Tables.Main)
	if err := r.insertBulk(tmain, r.Cfg.Tables.RecordsTags, r.Cfg.Tables.Tags, bs); err != nil {
		errRoll := tx.Rollback()
		return fmt.Errorf("%w: restoring bookmark, rollback %w", err, errRoll)
	}

	ids := slice.New[int]()
	bs.ForEach(func(b Row) {
		ids.Append(&b.ID)
	})

	if err := r.deleteBulk(r.Cfg.Tables.Main, ids); err != nil {
		errRoll := tx.Rollback()
		return fmt.Errorf("%w: restoring bookmark, rollback %w", err, errRoll)
	}

	return r.resetSQLiteSequence(r.Cfg.Tables.Deleted)
}
