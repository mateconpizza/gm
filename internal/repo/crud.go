package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/slice"
)

type (
	Row   = bookmark.Bookmark
	Slice = slice.Slice[Row]
)

// InsertOne creates a new record in the main table.
func (r *SQLiteRepository) InsertOne(ctx context.Context, b *Row) error {
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
}

// InsertMany creates multiple records.
func (r *SQLiteRepository) InsertMany(ctx context.Context, bs *Slice) error {
	return r.insertBulk(ctx, bs)
}

// DeleteOne deletes one record from the main table.
func (r *SQLiteRepository) DeleteOne(ctx context.Context, bURL string) error {
	return r.delete(ctx, bURL)
}

// DeleteMany deletes multiple records from the main table.
func (r *SQLiteRepository) DeleteMany(ctx context.Context, bs *Slice) error {
	if bs.Empty() {
		return ErrRecordIDNotProvided
	}
	log.Printf("deleting %d records from the relation table", bs.Len())
	var urls []string
	bs.ForEach(func(b Row) {
		urls = append(urls, b.URL)
	})

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		// create query
		q, args, err := sqlx.In("DELETE FROM bookmark_tags WHERE bookmark_url IN (?)", urls)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		// prepare statement
		stmt, err := tx.Preparex(q)
		if err != nil {
			return fmt.Errorf("delete many: %w: prepared statement", err)
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				log.Printf("delete many: %v: closing stmt", err)
			}
		}()
		// execute statement
		_, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return fmt.Errorf("delete many: %w: getting the result", err)
		}
		if err := stmt.Close(); err != nil {
			return fmt.Errorf("delete many: %w: closing stmt", err)
		}

		return nil
	})
}

// UpdateOne updates an existing record in the relation table.
func (r *SQLiteRepository) UpdateOne(ctx context.Context, newB, oldB *Row) (*Row, error) {
	if err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.delete(ctx, oldB.URL); err != nil {
			return fmt.Errorf("delete old record: %w", err)
		}
		newB.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := r.insertAtID(tx, newB); err != nil {
			return fmt.Errorf("insert new record: %w", err)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return newB, nil
}

// All returns all bookmarks.
func (r *SQLiteRepository) All(bs *Slice) error {
	log.Printf("getting all records")
	q := `
    SELECT
      b.*,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
    GROUP BY
      b.id
    ORDER BY
      b.id ASC;`
	if err := r.bySQL(bs, q); err != nil {
		return err
	}
	if bs.Len() == 0 {
		return ErrRecordNotFound
	}
	log.Printf("got %d records", bs.Len())

	return nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLiteRepository) ByID(bID int) (*Row, error) {
	if bID > r.maxID() {
		return nil, fmt.Errorf("%w. max: %d", ErrRecordNotFound, r.maxID())
	}
	log.Printf("getting record by ID=%d\n", bID)
	q := `
    SELECT
      b.*,
      COALESCE(
        GROUP_CONCAT(t.name, ','),
        ''
      ) AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
      b.id = ?`
	var b Row
	err := r.DB.Get(&b, q, bID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, bID)
		}

		return nil, fmt.Errorf("getting by ID: %w", err)
	}

	return &b, nil
}

// ByIDList returns a list of records by their IDs in the give table.
func (r *SQLiteRepository) ByIDList(bIDs []int, bs *Slice) error {
	if len(bIDs) == 0 {
		return ErrRecordIDNotProvided
	}
	q, args, err := sqlx.In(`
    SELECT
      b.*,
      COALESCE(
        GROUP_CONCAT(t.name, ','),
        ''
      ) AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
      b.id IN (?)
    GROUP BY
      b.id
    ORDER BY
      b.id ASC
    `, bIDs)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.bySQL(bs, r.DB.Rebind(q), args...)
}

// ByURL returns a record by its URL in the give table.
func (r *SQLiteRepository) ByURL(bURL string) (*Row, error) {
	row := r.DB.QueryRowx(`
    SELECT
      b.*,
      COALESCE(
        GROUP_CONCAT(t.name, ','),
        ''
      ) AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
      b.url = ?`, bURL)
	var b Row
	err := row.StructScan(&b)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with url: %s", ErrRecordNotFound, bURL)
		}

		return nil, fmt.Errorf("ByURL %w: %w", ErrRecordScan, err)
	}

	return &b, nil
}

// ByTag returns records filtered by tag, including all associated tags.
func (r *SQLiteRepository) ByTag(ctx context.Context, tag string, bs *Slice) error {
	q := `
    SELECT
      b.*,
      COALESCE(GROUP_CONCAT(t_all.name, ','), '') AS tags
    FROM bookmarks b
    JOIN bookmark_tags bt_filter ON b.url = bt_filter.bookmark_url
    JOIN tags t_filter ON bt_filter.tag_id = t_filter.id
    JOIN bookmark_tags bt_all ON b.url = bt_all.bookmark_url
    JOIN tags t_all ON bt_all.tag_id = t_all.id
    WHERE LOWER(t_filter.name) LIKE LOWER(?)
    GROUP BY b.id
    ORDER BY b.id ASC;`

	return r.bySQL(bs, q, "%"+tag+"%")
}

// ByQuery returns records by query in the give table.
func (r *SQLiteRepository) ByQuery(query string, bs *Slice) error {
	log.Printf("getting records by query: %q", query)
	q := `
    SELECT
      b.*,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM bookmarks b
    LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
    LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
        (LOWER(b.id || b.title || b.url || b.desc) LIKE LOWER(?) OR
        LOWER(t.name) LIKE LOWER(?))
      GROUP BY b.id
      ORDER BY b.id ASC;`
	queryValue := "%" + query + "%"
	if err := r.bySQL(bs, q, queryValue, queryValue); err != nil {
		return err
	}
	if bs.Len() == 0 {
		return ErrRecordNoMatch
	}
	log.Printf("got %d records by query: %q", bs.Len(), query)

	return nil
}

// Has checks if a record exists in the main table.
func (r *SQLiteRepository) Has(bURL string) (*Row, bool) {
	var count int
	q := "SELECT COUNT(*) FROM bookmarks WHERE url = ?"
	if err := r.DB.QueryRowx(q, bURL).Scan(&count); err != nil {
		log.Fatal(err)
	}
	if count == 0 {
		return nil, false
	}

	item, err := r.ByURL(bURL)
	if err != nil {
		return nil, false
	}

	return item, true
}

// ReorderIDs reorders the IDs in the main table.
func (r *SQLiteRepository) ReorderIDs(ctx context.Context) error {
	// get all records
	bs := slice.New[Row]()
	if err := r.All(bs); err != nil {
		return err
	}
	if bs.Empty() {
		return nil
	}

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		// drop the trigger to avoid errors during the table reorganization.
		if _, err := tx.Exec("DROP TRIGGER IF EXISTS cleanup_bookmark_and_tags"); err != nil {
			return fmt.Errorf("dropping trigger: %w", err)
		}
		// create temp table
		if err := r.tableCreate(tx, schemaTemp.name, schemaTemp.sql); err != nil {
			return err
		}
		// populate temp table
		if err := r.insertManyIntoTempTable(ctx, tx, bs); err != nil {
			return fmt.Errorf("%w: insert many (table %q)", err, schemaTemp.name)
		}
		// drop main table
		if err := r.tableDrop(tx, schemaMain.name); err != nil {
			return err
		}
		// rename temp table to main table
		if err := r.tableRename(tx, schemaTemp.name, schemaMain.name); err != nil {
			return err
		}
		// create index
		if _, err := tx.Exec(schemaTemp.index); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
		// restore relational table trigger
		if _, err := tx.Exec(schemaRelation.trigger); err != nil {
			return fmt.Errorf("recreating trigger: %w", err)
		}

		return nil
	})
}

// bySQL retrieves records from the SQLite database based on the provided SQL query.
func (r *SQLiteRepository) bySQL(bs *Slice, q string, args ...any) error {
	var bb []Row
	err := r.DB.Select(&bb, q, args...)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	bs.Set(&bb)
	bs.Sort(func(a, b Row) bool {
		return a.ID < b.ID
	})

	return nil
}

// DeleteOne deletes one record from the relation table.
func (r *SQLiteRepository) delete(ctx context.Context, bURL string) error {
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.Exec("DELETE FROM bookmark_tags WHERE bookmark_url = ?", bURL)
		if err != nil {
			return fmt.Errorf("failed to delete record: %w", err)
		}

		return nil
	})
}

// deleteOneTx deletes an single record in the given table.
func (r *SQLiteRepository) deleteOneTx(tx *sqlx.Tx, b *Row) error {
	log.Printf("deleting record: %s (table: bookmarks)", b.URL)
	// remove tags relationships first
	if _, err := tx.Exec(
		fmt.Sprintf("DELETE FROM %s WHERE bookmark_url = ?", schemaRelation.name),
		b.URL,
	); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}
	// remove main record
	result, err := tx.Exec("DELETE FROM bookmarks WHERE id = ?", b.ID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}
	// check results
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

// deleteAll deletes all records in the give table.
func (r *SQLiteRepository) deleteAll(ctx context.Context, ts ...Table) error {
	if len(ts) == 0 {
		log.Printf("no tables to delete")
		return nil
	}
	log.Printf("deleting all records from %d tables", len(ts))

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
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

// hasTx checks if a record exists in the specified table and column in
// a transaction.
func (r *SQLiteRepository) hasTx(tx *sqlx.Tx, target any) (bool, error) {
	var exists bool
	err := tx.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookmarks WHERE url = ?)", target)
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	return exists, nil
}

// insertAtID inserts a new record at the given ID.
func (r *SQLiteRepository) insertAtID(tx *sqlx.Tx, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}
	q := `
    INSERT
    OR IGNORE INTO bookmarks (id, url, title, desc, created_at, updated_at, visit_count, favorite)
    VALUES
    (:id, :url, :title, :desc, :created_at, :updated_at, :visit_count, :favorite)`
	_, err := tx.NamedExec(q, b)
	if err != nil {
		return fmt.Errorf("%w: %q", err, b.URL)
	}
	if err := r.associateTags(tx, b); err != nil {
		return fmt.Errorf("failed to associate tags: %w", err)
	}

	return nil
}

// insertBulk creates multiple records in the given tables.
func (r *SQLiteRepository) insertBulk(ctx context.Context, bs *Slice) error {
	log.Printf("inserting %d records into main table", bs.Len())
	bs.Sort(func(a, b Row) bool {
		return a.ID < b.ID
	})

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return bs.ForEachErr(func(b Row) error {
			return r.insertIntoTx(tx, &b)
		})
	})
}

// insertInto creates a new record in the given tables.
func (r *SQLiteRepository) insertInto(ctx context.Context, b *Row) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("insert record: %w", err)
	}
	if _, exists := r.Has(b.URL); exists {
		return ErrRecordDuplicate
	}
	// create record and associate tags
	err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := insertRecord(tx, b); err != nil {
			return err
		}
		if err := r.associateTags(tx, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("%w: %q", err, b.URL)
	}

	log.Printf("inserted record: %s\n", b.URL)

	return nil
}

// insertIntoTx inserts a record inside an existing transaction.
func (r *SQLiteRepository) insertIntoTx(tx *sqlx.Tx, b *Row) error {
	if _, err := r.hasTx(tx, b.URL); err != nil {
		return fmt.Errorf("duplicate record: %w, %q", err, b.URL)
	}
	// insert record and associate tags in the same transaction.
	if err := insertRecord(tx, b); err != nil {
		return err
	}
	if err := r.associateTags(tx, b); err != nil {
		return fmt.Errorf("failed to associate tags: %w", err)
	}
	log.Printf("inserted record: %s\n", b.URL)

	return nil
}

// insertManyIntoTempTable inserts multiple records into a temporary table.
func (r *SQLiteRepository) insertManyIntoTempTable(
	ctx context.Context,
	tx *sqlx.Tx,
	bs *Slice,
) error {
	q := `
  INSERT INTO temp_bookmarks (
    url, title, desc, created_at, last_visit,
    updated_at, visit_count, favorite
  )
  VALUES
    (
      :url, :title, :desc, :created_at, :last_visit,
      :updated_at, :visit_count, :favorite
    )
  `
	// FIX: pass the context
	stmt, err := tx.PrepareNamedContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("delete many: %v: closing stmt", err)
		}
	}()
	insertter := func(b Row) error {
		if _, err := stmt.Exec(b); err != nil {
			return fmt.Errorf("insert bookmark %s: %w", b.URL, err)
		}

		return nil
	}
	if err := bs.ForEachErr(insertter); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// insertRecord inserts a new record into the table.
func insertRecord(tx *sqlx.Tx, b *Row) error {
	r, err := tx.NamedExec(
		`INSERT INTO bookmarks (
    url, title, desc, created_at, last_visit,
    updated_at, visit_count, favorite
  )
  VALUES
    (
      :url, :title, :desc, :created_at, :last_visit,
      :updated_at, :visit_count, :favorite
    )`,
		&b,
	)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	bid, err := r.LastInsertId()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	b.ID = int(bid)

	return nil
}

// maxID retrieves the maximum ID from the specified table in the SQLite
// database.
func (r *SQLiteRepository) maxID() int {
	var lastIndex int
	if err := r.DB.QueryRowx("SELECT COALESCE(MAX(id), 0) FROM bookmarks").Scan(&lastIndex); err != nil {
		return 0
	}

	return lastIndex
}

// withTx executes a function within a transaction.
func (r *SQLiteRepository) withTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // ensure rollback on panic
			panic(p)          // re-throw the panic after rollback
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
