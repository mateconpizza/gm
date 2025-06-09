package repo

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/slice"
)

// InsertOne creates a new record in the main table.
func (r *SQLiteRepository) InsertOne(ctx context.Context, b *bookmark.Bookmark) error {
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return r.insertIntoTx(tx, b)
	})
}

// InsertMany creates multiple records.
func (r *SQLiteRepository) InsertMany(ctx context.Context, bs *slice.Slice[bookmark.Bookmark]) error {
	return r.insertBulk(ctx, bs)
}

// DeleteOne deletes one record from the main table.
func (r *SQLiteRepository) DeleteOne(ctx context.Context, bURL string) error {
	return r.delete(ctx, bURL)
}

// DeleteMany deletes multiple records from the main table.
func (r *SQLiteRepository) DeleteMany(ctx context.Context, bs *slice.Slice[bookmark.Bookmark]) error {
	if bs.Empty() {
		return ErrRecordIDNotProvided
	}
	slog.Debug("deleting many records from the relation table", "count", bs.Len())
	var urls []string
	bs.ForEach(func(b bookmark.Bookmark) {
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
				slog.Error("delete many: closing stmt", "error", err)
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

// Update updates an existing record in the relation table.
func (r *SQLiteRepository) Update(
	ctx context.Context,
	newB, oldB *bookmark.Bookmark,
) (*bookmark.Bookmark, error) {
	if err := r.withTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.delete(ctx, oldB.URL); err != nil {
			return fmt.Errorf("delete old record: %w", err)
		}
		newB.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		newB.GenerateChecksum()
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
func (r *SQLiteRepository) All() ([]bookmark.Bookmark, error) {
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
	bs, err := r.bySQL(q)
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		return nil, ErrRecordNotFound
	}
	slog.Debug("getting all records", "got", len(bs))

	return bs, nil
}

// AllPtr returns all bookmarks.
func (r *SQLiteRepository) AllPtr() ([]*bookmark.Bookmark, error) {
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
	bs, err := r.bySQLPtr(q)
	if err != nil {
		return nil, err
	}
	slog.Debug("getting all records", "got", len(bs))

	return bs, nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLiteRepository) ByID(bID int) (*bookmark.Bookmark, error) {
	if bID > r.maxID() {
		return nil, fmt.Errorf("%w. max: %d", ErrRecordNotFound, r.maxID())
	}
	slog.Info("getting record by ID", "id", bID)
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
	var b bookmark.Bookmark
	err := r.DB.Get(&b, q, bID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w with id: %d", ErrRecordNotFound, bID)
		}

		return nil, fmt.Errorf("getting by ID: %w", err)
	}

	b.Tags = bookmark.ParseTags(b.Tags)

	return &b, nil
}

// ByIDList returns a list of records by their IDs in the give table.
func (r *SQLiteRepository) ByIDList(bIDs []int) ([]bookmark.Bookmark, error) {
	if len(bIDs) == 0 {
		return nil, ErrRecordIDNotProvided
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
		return nil, fmt.Errorf("%w", err)
	}

	bs, err := r.bySQL(r.DB.Rebind(q), args...)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// ByURL returns a record by its URL in the give table.
func (r *SQLiteRepository) ByURL(bURL string) (*bookmark.Bookmark, error) {
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
	var b bookmark.Bookmark
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
func (r *SQLiteRepository) ByTag(ctx context.Context, tag string) ([]bookmark.Bookmark, error) {
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

	bs, err := r.bySQL(q, "%"+tag+"%")
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// ByQuery returns records by query in the give table.
func (r *SQLiteRepository) ByQuery(query string) ([]bookmark.Bookmark, error) {
	slog.Info("getting records by query", "query", query)
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
	bs, err := r.bySQL(q, queryValue, queryValue)
	if err != nil {
		return nil, err
	}
	if len(bs) == 0 {
		return nil, ErrRecordNoMatch
	}
	slog.Info("got records by query", "count", len(bs), "query", query)

	return bs, nil
}

// Has checks if a record exists in the main table.
func (r *SQLiteRepository) Has(bURL string) (*bookmark.Bookmark, bool) {
	var count int
	q := "SELECT COUNT(*) FROM bookmarks WHERE url = ?"
	if err := r.DB.QueryRowx(q, bURL).Scan(&count); err != nil {
		slog.Error("error getting count", "error", err)
		r.Close()
		os.Exit(1)
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
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		// check if last item has been deleted
		if r.maxID() == 0 {
			return resetSQLiteSequence(tx, schemaMain.name)
		}
		// get all records
		bb, err := r.All()
		if err != nil {
			if !errors.Is(ErrRecordNotFound, err) {
				return err
			}
		}
		bs := slice.New[bookmark.Bookmark]()
		bs.Set(&bb)
		if bs.Empty() {
			return nil
		}
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
func (r *SQLiteRepository) bySQL(q string, args ...any) ([]bookmark.Bookmark, error) {
	var bb []bookmark.Bookmark
	err := r.DB.Select(&bb, q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	slices.SortFunc(bb, func(a, b bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})
	for i := range bb {
		bb[i].Tags = bookmark.ParseTags(bb[i].Tags)
	}

	return bb, nil
}

// bySQL retrieves records from the SQLite database based on the provided SQL query.
func (r *SQLiteRepository) bySQLPtr(q string, args ...any) ([]*bookmark.Bookmark, error) {
	var bb []*bookmark.Bookmark
	err := r.DB.Select(&bb, q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	slices.SortFunc(bb, func(a, b *bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})
	for i := range bb {
		bb[i].Tags = bookmark.ParseTags(bb[i].Tags)
	}

	return bb, nil
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
func (r *SQLiteRepository) deleteOneTx(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	slog.Debug("deleting record", "url", b.URL)
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
	slog.Debug("deleted record", "id", b.ID)

	return nil
}

// deleteAll deletes all records in the give table.
func (r *SQLiteRepository) deleteAll(ctx context.Context, ts ...Table) error {
	if len(ts) == 0 {
		slog.Debug("no tables to delete")
		return nil
	}
	slog.Debug("deleting all records from tables", "tables", ts)

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		for _, t := range ts {
			slog.Debug("deleting records from table", "table", t)
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
func (r *SQLiteRepository) insertAtID(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}
	q := `
    INSERT
    OR IGNORE INTO bookmarks (id, url, title, desc, created_at, updated_at, visit_count, favorite, checksum)
    VALUES
    (:id, :url, :title, :desc, :created_at, :updated_at, :visit_count, :favorite, :checksum)`
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
func (r *SQLiteRepository) insertBulk(ctx context.Context, bs *slice.Slice[bookmark.Bookmark]) error {
	slog.Info("inserting records into main table", "count", bs.Len())
	bs.Sort(func(a, b bookmark.Bookmark) bool {
		return a.ID < b.ID
	})

	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return bs.ForEachErr(func(b bookmark.Bookmark) error {
			return r.insertIntoTx(tx, &b)
		})
	})
}

// insertInto creates a new record in the given tables.
func (r *SQLiteRepository) insertInto(ctx context.Context, b *bookmark.Bookmark) error {
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

	slog.Debug("inserted record", "url", b.URL)

	return nil
}

// insertIntoTx inserts a record inside an existing transaction.
func (r *SQLiteRepository) insertIntoTx(tx *sqlx.Tx, b *bookmark.Bookmark) error {
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
	slog.Debug("inserted record", "url", b.URL)

	return nil
}

// insertManyIntoTempTable inserts multiple records into a temporary table.
func (r *SQLiteRepository) insertManyIntoTempTable(
	ctx context.Context,
	tx *sqlx.Tx,
	bs *slice.Slice[bookmark.Bookmark],
) error {
	q := `
  INSERT INTO temp_bookmarks (
    url, title, desc, created_at, last_visit,
    updated_at, visit_count, favorite, checksum
  )
  VALUES
    (
      :url, :title, :desc, :created_at, :last_visit,
      :updated_at, :visit_count, :favorite, :checksum
    )
  `
	// FIX: pass the context
	stmt, err := tx.PrepareNamedContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("delete many: closing stmt", "error", err)
		}
	}()
	insertter := func(b bookmark.Bookmark) error {
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
func insertRecord(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	if b.Checksum == "" {
		b.GenerateChecksum()
	}
	b.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	r, err := tx.NamedExec(
		`INSERT INTO bookmarks (
    	url, title, desc, created_at, last_visit,
    	updated_at, visit_count, favorite, checksum
  )
  VALUES
    (
      :url, :title, :desc, :created_at, :last_visit,
			:updated_at, :visit_count, :favorite, :checksum
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
			slog.Error("rollback error", "error", err)
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

func (r *SQLiteRepository) UpdateVisitDateAndCount(ctx context.Context, b *bookmark.Bookmark) error {
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return updateVisit(tx, b)
	})
}

func (r *SQLiteRepository) Favorite(ctx context.Context, b *bookmark.Bookmark) error {
	return r.withTx(ctx, func(tx *sqlx.Tx) error {
		return updateFavorite(tx, b)
	})
}

// updateVisit updates the visit count and last visit date for a bookmark.
func updateVisit(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	_, err := tx.Exec(
		"UPDATE bookmarks SET visit_count = visit_count + 1, last_visit = ? WHERE url = ?",
		time.Now().UTC().Format(time.RFC3339),
		b.URL,
	)
	if err != nil {
		return fmt.Errorf("failed to update visit count: %w", err)
	}

	return nil
}

func updateFavorite(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	_, err := tx.Exec(
		"UPDATE bookmarks SET favorite = ? WHERE url = ?",
		b.Favorite,
		b.URL,
	)
	if err != nil {
		return fmt.Errorf("failed to update favorite: %w", err)
	}

	return nil
}
