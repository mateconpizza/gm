package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/slice"
)

type (
	Row   = bookmark.Bookmark
	Slice = slice.Slice[Row]
	Table string
)

// Insert creates a new record in the main table.
func (r *SQLiteRepository) Insert(b *Row) error {
	t := r.Cfg.Tables
	return r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, t.Main, t.RecordsTags, b)
	})
}

// insertInto creates a new record in the given tables.
func (r *SQLiteRepository) insertInto(ctx context.Context, tmain, trecords Table, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}
	if r.HasRecord(tmain, "url", b.URL) {
		return ErrRecordDuplicate
	}
	// create record and associate tags
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		if err := insertRecord(tx, tmain, b); err != nil {
			return err
		}

		if err := r.associateTags(tx, trecords, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("%w: '%s'", err, b.URL)
	}

	log.Printf("inserted record: %s (table: %s)\n", b.URL, tmain)

	return nil
}

// insertIntoTx inserts a record inside an existing transaction.
func (r *SQLiteRepository) insertIntoTx(tx *sqlx.Tx, tmain, trecords Table, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	if _, err := r.hasRecordInTx(tx, tmain, "url", b.URL); err != nil {
		return ErrRecordDuplicate
	}
	// insert record and associate tags in the same transaction.
	if err := insertRecord(tx, tmain, b); err != nil {
		return err
	}
	if err := r.associateTags(tx, trecords, b); err != nil {
		return fmt.Errorf("failed to associate tags: %w", err)
	}
	log.Printf("inserted record: %s (table: %s)\n", b.URL, tmain)

	return nil
}

// insertAtID inserts a new record at the given ID.
func (r *SQLiteRepository) insertAtID(tx *sqlx.Tx, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}
	if r.HasRecord(r.Cfg.Tables.Main, "id", b.ID) {
		return ErrRecordDuplicate
	}

	query := fmt.Sprintf(`
    INSERT OR IGNORE INTO %s
    (id, url, title, desc)
    VALUES
    (:id, :url, :title, :desc)`, r.Cfg.Tables.Main)

	_, err := tx.NamedExec(query, &b)
	if err != nil {
		return fmt.Errorf("%w: '%s'", ErrRecordInsert, b.URL)
	}

	if err := r.associateTags(tx, r.Cfg.Tables.RecordsTags, b); err != nil {
		return fmt.Errorf("InsertWithID: %w", err)
	}

	return nil
}

// insertRecord inserts a new record into the table.
func insertRecord(tx *sqlx.Tx, t Table, b *Row) error {
	q := fmt.Sprintf(`
    INSERT or IGNORE INTO %s
    (url, title, desc)
    VALUES (:url, :title, :desc)`, t)
	result, err := tx.NamedExec(q, &b)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bid, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	b.ID = int(bid)

	return nil
}

// InsertMultiple creates multiple records.
func (r *SQLiteRepository) InsertMultiple(bs *Slice) error {
	return r.insertBulk(context.Background(), r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTags, bs)
}

// insertBulkNNew creates multiple records in the given tables.
func (r *SQLiteRepository) insertBulk(ctx context.Context, tmain, trecords Table, bs *Slice) error {
	log.Printf("inserting %d records into table: %s", bs.Len(), tmain)
	return r.execTx(ctx, func(tx *sqlx.Tx) error {
		return bs.ForEachErr(func(b Row) error {
			return r.insertIntoTx(tx, tmain, trecords, &b)
		})
	})
}

// Update updates an existing record in the given table.
func (r *SQLiteRepository) Update(b *Row) (*Row, error) {
	t := r.Cfg.Tables.Main
	if !r.HasRecord(t, "url", b.URL) || !r.HasRecord(t, "id", b.ID) {
		return b, fmt.Errorf("%w: in updating '%s'", ErrRecordNotExists, b.URL)
	}
	err := r.execTx(context.Background(), func(tx *sqlx.Tx) error {
		q := fmt.Sprintf(`
    UPDATE %s
    SET
        url = :url,
        title = :title,
        desc = :desc
    WHERE
        id = :id`, t)
		_, err := r.DB.NamedExec(q, &b)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrRecordUpdate, err)
		}
		if err := r.updateTags(tx, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	log.Printf("updated record %s (table: %s)\n", b.URL, t)

	return b, nil
}

// UpdateURL updates the URL of an existing record.
func (r *SQLiteRepository) UpdateURL(newB, oldB *Row) (*Row, error) {
	// FIX: redo this logic
	t := r.Cfg.Tables.Main
	ctx := context.Background()
	// first remove the old record
	if err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.deleteInTx(tx, t, oldB)
	}); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	// then insert the new record
	if err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertAtID(tx, newB)
	}); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return newB, nil
}

// delete deletes an single record in the given table.
func (r *SQLiteRepository) delete(ctx context.Context, t Table, b *Row) error {
	return r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.deleteInTx(tx, t, b)
	})
}

// deleteInTx deletes an single record in the given table.
func (r *SQLiteRepository) deleteInTx(tx *sqlx.Tx, t Table, b *Row) error {
	log.Printf("deleting record: %s (table: %s)", b.URL, t)

	// 1. Eliminar relaciones de tags primero
	if _, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE bookmark_url = ?", r.Cfg.Tables.RecordsTags),
		b.URL,
	); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}

	// 2. Eliminar registro principal
	result, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE id = ?", t),
		b.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	// 3. Verificar que se afectó exactamente 1 fila
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	log.Printf("successfully deleted record ID %d", b.ID)

	return nil
}

// delete deletes an single record in the given table.
// func (r *SQLiteRepository) delete(ctx context.Context, t Table, b *Row) error {
// 	log.Printf("deleting record: %s (table: %s)\n", b.URL, t)
// 	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
// 		tbs := r.Cfg.Tables
// 		_, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tbs.Main), b.ID)
// 		if err != nil {
// 			return fmt.Errorf("%w", err)
// 		}
// 		if err := deleteTags(tx, tbs.RecordsTags, b.URL); err != nil {
// 			return fmt.Errorf("%w", err)
// 		}
//
// 		return nil
// 	})
// 	if err != nil {
// 		return fmt.Errorf("Delete: %w", err)
// 	}
//
// 	log.Printf("deleted record with ID %d from table: %s", b.ID, t)
//
// 	return nil
// }

// deleteAll deletes all records in the give table.
func (r *SQLiteRepository) deleteAll(ctx context.Context, ts ...Table) error {
	if len(ts) == 0 {
		log.Printf("no tables to delete")
		return nil
	}
	log.Printf("deleting all records from %d tables", len(ts))

	return r.execTx(ctx, func(tx *sqlx.Tx) error {
		for _, t := range ts {
			log.Printf("deleting records from table: %s", t)
			_, err := tx.Exec(fmt.Sprintf("DELETE FROM %s", t))
			if err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		return nil
	})
}

// deleteBulk deletes multiple records in the give table.
func (r *SQLiteRepository) deleteBulk(ctx context.Context, t Table, ids *slice.Slice[int]) error {
	n := ids.Len()
	if n == 0 {
		return ErrRecordIDNotProvided
	}
	log.Printf("deleting %d records from table: %s", n, t)
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		query := fmt.Sprintf("DELETE FROM %s WHERE id IN (?)", t)
		q, args, err := sqlx.In(query, *ids.Items())
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		// prepare statement
		stmt, err := tx.Preparex(q)
		if err != nil {
			return fmt.Errorf("DeleteBulk: %w: prepared statement", err)
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				log.Printf("DeleteBulk: %v: closing stmt", err)
			}
		}()
		// execute statement
		_, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return fmt.Errorf("DeleteBulk: %w: getting the result", err)
		}
		if err := stmt.Close(); err != nil {
			return fmt.Errorf("DeleteBulk: %w: closing stmt", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	log.Printf("deleted %d records from table: %s", n, t)

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
		ids.Append(b.ID)
	})
	if ids.Empty() {
		return ErrRecordIDNotProvided
	}
	ctx := context.Background()
	if err := r.deleteBulk(ctx, main, ids); err != nil {
		return fmt.Errorf("deleting records in bulk: %w", err)
	}
	// add records to deleted table
	if err := r.insertBulk(ctx, deleted, r.Cfg.Tables.RecordsTagsDeleted, bs); err != nil {
		if errors.Is(err, ErrRecordDuplicate) {
			return nil
		}

		return fmt.Errorf("inserting records in bulk after deletion: %w", err)
	}
	// if the last record is deleted, we don't need to reorder
	// reset the SQLite sequence
	if r.maxID(main) == 0 {
		err := r.execTx(ctx, func(tx *sqlx.Tx) error {
			err := r.resetSQLiteSequence(tx, main, deleted)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		})

		return err
	}
	if err := r.reorderIDs(ctx, main); err != nil {
		return fmt.Errorf("reordering ids: %w", err)
	}

	return nil
}

// Records returns all records in the give table.
func (r *SQLiteRepository) Records(t Table, bs *Slice) error {
	log.Printf("getting all records from table: '%s'", t)
	if err := r.bySQL(bs, fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", t)); err != nil {
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
func (r *SQLiteRepository) ByID(t Table, bID int) (*Row, error) {
	if bID > r.maxID(t) {
		return nil, fmt.Errorf("%w. max: %d", ErrRecordNotFound, r.maxID(t))
	}
	log.Printf("getting record by ID=%d (table: %s)\n", bID, t)
	var b Row
	q := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", t)
	err := r.DB.Get(&b, q, bID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, bID)
		}

		return nil, fmt.Errorf("ByID: %w: %w", ErrRecordScan, err)
	}
	if err := r.loadRecordTags(&b); err != nil {
		return nil, err
	}

	log.Printf("got record by ID %d (table: %s)\n", bID, t)

	return &b, nil
}

// ByIDList returns a list of records by their IDs in the give table.
func (r *SQLiteRepository) ByIDList(t Table, bIDs []int, bs *Slice) error {
	if len(bIDs) == 0 {
		return ErrRecordIDNotProvided
	}
	q, args, err := sqlx.In(fmt.Sprintf("SELECT * FROM %s WHERE ID IN (?)", t), bIDs)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.bySQL(bs, r.DB.Rebind(q), args...)
}

// ByURL returns a record by its URL in the give table.
func (r *SQLiteRepository) ByURL(t Table, bURL string) (*Row, error) {
	row := r.DB.QueryRowx(fmt.Sprintf("SELECT * FROM %s WHERE url = ?", t), bURL)
	var b Row
	err := row.StructScan(&b)
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
func (r *SQLiteRepository) bySQL(bs *Slice, q string, args ...any) error {
	var bb []bookmark.Bookmark
	err := r.DB.Select(&bb, q, args...)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	bs.Set(&bb)
	if err := r.populateTags(bs); err != nil {
		return err
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
	if bs.Len() == 0 {
		return ErrRecordNoMatch
	}
	log.Printf("got %d records by query: '%s'", bs.Len(), q)

	return nil
}

// ByColumn returns the data found from the given column name.
func (r *SQLiteRepository) ByColumn(t Table, c string) (*slice.Slice[string], error) {
	log.Printf("getting all records from table: '%s' and column: '%s'", t, c)
	items := make([]string, 0)
	if err := r.DB.Select(&items, fmt.Sprintf("SELECT %s FROM %s ORDER BY id ASC", c, t)); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	data := slice.New[string]()
	data.Set(&items)

	n := data.Len()
	if n == 0 {
		log.Printf("no tags found in table: '%s' and column: '%s'", t, c)
		return nil, fmt.Errorf("%w by table: '%s' and column: '%s'", ErrRecordNotFound, t, c)
	}

	log.Printf("tags found: %d by column: '%s'", n, c)

	return data, nil
}

// HasRecord checks if a record exists in the specified table and column.
func (r *SQLiteRepository) HasRecord(t Table, column, target any) bool {
	var recordCount int
	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s=?", t, column)
	if err := r.DB.QueryRowx(sqlQuery, target).Scan(&recordCount); err != nil {
		log.Fatal(err)
	}

	return recordCount > 0
}

// hasRecordInTx checks if a record exists in the specified table and column in
// a transaction.
func (r *SQLiteRepository) hasRecordInTx(tx *sqlx.Tx, t Table, column, target any) (bool, error) {
	var exists bool
	q := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = ?)", t, column)
	err := tx.Get(&exists, q, target)
	if err != nil {
		return false, fmt.Errorf("hasRecordInTx: %w", err)
	}

	return exists, nil
}

// maxID retrieves the maximum ID from the specified table in the SQLite
// database.
func (r *SQLiteRepository) maxID(t Table) int {
	var lastIndex int
	q := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", t)
	if err := r.DB.QueryRowx(q).Scan(&lastIndex); err != nil {
		return 0
	}

	return lastIndex
}

// reorderIDs reorders the IDs in the specified table.
func (r *SQLiteRepository) reorderIDs(ctx context.Context, t Table) error {
	// FIX: Every time we re-order IDs, the db's size gets bigger
	// It's a bad implementation? (but it works)
	// Maybe use 'VACUUM' command? it is safe?
	log.Printf("reordering IDs in table: %s", t)
	bs := slice.New[Row]()
	if err := r.Records(t, bs); err != nil {
		return err
	}

	if bs.Empty() {
		return nil
	}

	tempTable := "temp_" + t
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		return r.tableCreate(tx, tempTable, tableMainSchema)
	})
	if err != nil {
		return err
	}

	return r.execTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.insertBulk(ctx, tempTable, r.Cfg.Tables.RecordsTags, bs); err != nil {
			return err
		}
		if err := r.tableDrop(tx, t); err != nil {
			return err
		}

		return r.tableRename(tx, tempTable, t)
	})
}

// Restore restores record/s from deleted tabled.
func (r *SQLiteRepository) Restore(ctx context.Context, from, to Table, bs *Slice) error {
	err := r.execTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.insertBulk(ctx, to, r.Cfg.Tables.RecordsTags, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		ids := slice.New[int]()
		bs.ForEach(func(b Row) {
			ids.Append(b.ID)
		})

		// delete records from deleted table
		if err := r.deleteBulk(ctx, from, ids); err != nil {
			return fmt.Errorf("%w", err)
		}

		// reset sqlite sequence
		if err := r.resetSQLiteSequence(tx, from); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	})

	return err
}

// execTx executes a function within a transaction.
func (r *SQLiteRepository) execTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // Ensure rollback on panic
			panic(p)          // Re-throw the panic after rollback
		} else if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			log.Printf("rollback error: %v", err)
		}
	}()

	if err := fn(tx); err != nil {
		return fmt.Errorf("fn transaction: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}
