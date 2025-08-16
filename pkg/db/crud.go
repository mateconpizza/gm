//nolint:perfsprint //ignore
package db

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

// InsertOne creates a new record in the main table.
func (r *SQLite) InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error) {
	var id int64
	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		id, err = r.insertIntoTx(tx, b)
		return err
	})

	return id, err
}

func (r *SQLite) InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	return r.insertBulkPtr(ctx, bs)
}

// DeleteMany deletes multiple records from the main table.
func (r *SQLite) DeleteMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	n := len(bs)
	if n == 0 {
		return ErrRecordIDNotProvided
	}

	slog.Debug("deleting many records from the relation table", "count", n)

	urls := make([]string, 0, len(bs))
	for i := range bs {
		urls = append(urls, bs[i].URL)
	}

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
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
func (r *SQLite) Update(ctx context.Context, newB, oldB *bookmark.Bookmark) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteOneTx(tx, oldB); err != nil {
			return fmt.Errorf("delete old record: %w", err)
		}

		newB.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := r.insertAtID(tx, newB); err != nil {
			return fmt.Errorf("insert new record: %w", err)
		}

		return nil
	})
}

// All returns all bookmarks.
func (r *SQLite) All(ctx context.Context) ([]*bookmark.Bookmark, error) {
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

	bs, err := r.bySQL(ctx, q)
	if err != nil {
		return nil, err
	}

	slog.Debug("getting all records", "got", len(bs))

	return bs, nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLite) ByID(ctx context.Context, bID int) (*bookmark.Bookmark, error) {
	if bID > r.MaxID(ctx) {
		return nil, fmt.Errorf("%w. max: %d", ErrRecordNotFound, r.MaxID(ctx))
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
	err := r.DB.GetContext(ctx, &b, q, bID)
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
func (r *SQLite) ByIDList(ctx context.Context, bIDs []int) ([]*bookmark.Bookmark, error) {
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

	bs, err := r.bySQL(ctx, r.DB.Rebind(q), args...)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// ByURL returns a record by its URL in the give table.
func (r *SQLite) ByURL(ctx context.Context, bURL string) (*bookmark.Bookmark, error) {
	row := r.DB.QueryRowxContext(ctx, `
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
func (r *SQLite) ByTag(ctx context.Context, tag string) ([]*bookmark.Bookmark, error) {
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

	bs, err := r.bySQL(ctx, q, "%"+tag+"%")
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// ByQuery returns records by query in the give table.
func (r *SQLite) ByQuery(ctx context.Context, query string) ([]*bookmark.Bookmark, error) {
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

	bs, err := r.bySQL(ctx, q, queryValue, queryValue)
	if err != nil {
		return nil, err
	}

	// FIX: remove
	if len(bs) == 0 {
		return nil, ErrRecordNoMatch
	}

	slog.Info("got records by query", "count", len(bs), "query", query)

	return bs, nil
}

// MaxID retrieves the maximum ID from the specified table in the SQLite
// database.
func (r *SQLite) MaxID(ctx context.Context) int {
	var lastIndex int
	if err := r.DB.QueryRowxContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM bookmarks").Scan(&lastIndex); err != nil {
		return 0
	}

	return lastIndex
}

// WithTx executes a function within a transaction.
func (r *SQLite) WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // ensure rollback on panic

			panic(p) // re-throw the panic after rollback
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

func (r *SQLite) AddVisit(ctx context.Context, bID int) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		return updateVisit(tx, bID)
	})
}

func (r *SQLite) SetFavorite(ctx context.Context, b *bookmark.Bookmark) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		return updateFavorite(tx, b)
	})
}

// FavoritesList returns the favorite bookmarks.
func (r *SQLite) FavoritesList(ctx context.Context) ([]*bookmark.Bookmark, error) {
	q := `
    SELECT
      b.*,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.url = bt.bookmark_url
      LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
      b.favorite = 1
    GROUP BY
      b.id
    ORDER BY
      b.id ASC;`

	return r.bySQL(ctx, q)
}

func (r *SQLite) ByOrder(ctx context.Context, column, sortBy string) ([]*bookmark.Bookmark, error) {
	sortBy = strings.ToUpper(sortBy)
	if sortBy != "ASC" && sortBy != "DESC" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidSortBy, sortBy)
	}

	q := fmt.Sprintf(`
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
      b.%s %s;`, column, sortBy)

	var bb []*bookmark.Bookmark
	err := r.DB.SelectContext(ctx, &bb, q)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	for _, b := range bb {
		b.Tags = bookmark.ParseTags(b.Tags)
	}

	return bb, nil
}

// Count returns the number of records in the given table.
func (r *SQLite) Count(ctx context.Context, table string) int {
	var n int
	q := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	if err := r.DB.QueryRowxContext(ctx, q).Scan(&n); err != nil {
		return 0
	}

	return n
}

// CountFavorites returns the number of favorite records.
func (r *SQLite) CountFavorites(ctx context.Context) int {
	var n int
	if err := r.DB.QueryRowxContext(ctx, "SELECT COUNT(*) FROM bookmarks WHERE favorite = 1").Scan(&n); err != nil {
		return 0
	}

	return n
}

// Has checks if a record exists in the main table.
func (r *SQLite) Has(ctx context.Context, bURL string) (*bookmark.Bookmark, bool) {
	var count int
	if err := r.DB.QueryRowxContext(ctx, "SELECT COUNT(*) FROM bookmarks WHERE url = ?", bURL).Scan(&count); err != nil {
		slog.Error("error getting count", "error", err)
		r.Close()
		os.Exit(1)
	}

	if count == 0 {
		return nil, false
	}

	item, err := r.ByURL(ctx, bURL)
	if err != nil {
		return nil, false
	}

	return item, true
}

// bySQL retrieves records from the SQLite database based on the provided SQL query.
func (r *SQLite) bySQL(ctx context.Context, q string, args ...any) ([]*bookmark.Bookmark, error) {
	var bb []*bookmark.Bookmark
	err := r.DB.SelectContext(ctx, &bb, q, args...)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	slices.SortFunc(bb, func(a, b *bookmark.Bookmark) int {
		return cmp.Compare(a.ID, b.ID)
	})

	for _, b := range bb {
		b.Tags = bookmark.ParseTags(b.Tags)
	}

	return bb, nil
}

// deleteOneTx deletes an single record in the given table.
func (r *SQLite) deleteOneTx(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	slog.Debug("deleting record", "url", b.URL)
	ctx := context.Background()

	// remove tags relationships first
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s WHERE bookmark_url = ?", schemaRelation.Name),
		b.URL,
	); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}

	// remove main record
	_, err := tx.ExecContext(ctx, "DELETE FROM bookmarks WHERE id = ?", b.ID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	slog.Debug("deleted record", "id", b.ID)

	return nil
}

// deleteAll deletes all records in the give table.
func (r *SQLite) deleteAll(ctx context.Context, ts ...Table) error {
	if len(ts) == 0 {
		slog.Debug("no tables to delete")
		return nil
	}

	slog.Debug("deleting all records from tables", "tables", ts)

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, t := range ts {
			slog.Debug("deleting records from table", "table", t)

			_, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", t))
			if err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		return nil
	})
}

// hasTx checks if a record exists in the specified table and column in
// a transaction.
func (r *SQLite) hasTx(tx *sqlx.Tx, target any) (bool, error) {
	var exists bool

	err := tx.Get(&exists, "SELECT EXISTS(SELECT 1 FROM bookmarks WHERE url = ?)", target)
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	return exists, nil
}

// insertAtID inserts a new record at the given ID.
func (r *SQLite) insertAtID(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("abort: %w", err)
	}

	q := `
	INSERT OR IGNORE INTO bookmarks (
		id,
		url,
		title,
		desc,
		created_at,
		updated_at,
		last_visit,
		visit_count,
		favorite,
		checksum,
		favicon_url,
		archive_url,
		archive_timestamp,
		favicon_local,
		status_code,
		status_text,
		is_active,
		last_checked
	) VALUES (
		:id,
		:url,
		:title,
		:desc,
		:created_at,
		:updated_at,
		:last_visit,
		:visit_count,
		:favorite,
		:checksum,
		:favicon_url,
		:archive_url,
		:archive_timestamp,
		:favicon_local,
		:status_code,
		:status_text,
		:is_active,
		:last_checked
	)`

	_, err := tx.NamedExec(q, b)
	if err != nil {
		return fmt.Errorf("%w: %q", err, b.URL)
	}

	if err := r.associateTags(tx, b); err != nil {
		return fmt.Errorf("failed to associate tags: %w", err)
	}

	return nil
}

func (r *SQLite) insertBulkPtr(ctx context.Context, bs []*bookmark.Bookmark) error {
	slog.Info("inserting records into main table", "count", len(bs))
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].ID < bs[j].ID
	})

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, b := range bs {
			if _, err := r.insertIntoTx(tx, b); err != nil {
				return err
			}
		}

		return nil
	})
}

// insertInto creates a new record in the given tables.
func (r *SQLite) insertInto(ctx context.Context, b *bookmark.Bookmark) error {
	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("insert record: %w", err)
	}

	if _, exists := r.Has(ctx, b.URL); exists {
		return ErrRecordDuplicate
	}
	// create record and associate tags
	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if _, err := insertRecord(tx, b); err != nil {
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
func (r *SQLite) insertIntoTx(tx *sqlx.Tx, b *bookmark.Bookmark) (int64, error) {
	b.URL = strings.TrimSuffix(b.URL, "/")
	if _, err := r.hasTx(tx, b.URL); err != nil {
		return 0, fmt.Errorf("duplicate record: %w, %q", err, b.URL)
	}

	if b.Checksum == "" {
		b.GenChecksum()
	}

	// insert record and associate tags in the same transaction.
	bID, err := insertRecord(tx, b)
	if err != nil {
		return 0, err
	}

	if err := r.associateTags(tx, b); err != nil {
		return 0, fmt.Errorf("failed to associate tags: %w", err)
	}

	slog.Debug("inserted record", "url", b.URL)

	return bID, nil
}

// insertRecord inserts a new record into the table.
func insertRecord(tx *sqlx.Tx, b *bookmark.Bookmark) (int64, error) {
	if b.Checksum == "" {
		return 0, ErrChecksumEmpty
	}

	b.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	r, err := tx.NamedExec(`
	INSERT INTO bookmarks (
		url,
		title,
		desc,
		created_at,
		last_visit,
		updated_at,
		visit_count,
		favorite,
		checksum,
		favicon_url,
		favicon_local,
		archive_url,
		archive_timestamp
	)
	VALUES (
		:url,
		:title,
		:desc,
		:created_at,
		:last_visit,
		:updated_at,
		:visit_count,
		:favorite,
		:checksum,
		:favicon_url,
		:favicon_local,
		:archive_url,
		:archive_timestamp
	)`, &b)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}

	bid, err := r.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}

	b.ID = int(bid)

	return bid, nil
}

// updateVisit updates the visit count and last visit date for a bookmark.
func updateVisit(tx *sqlx.Tx, bID int) error {
	slog.Debug("updating visit count", "id", bID)
	_, err := tx.ExecContext(
		context.Background(),
		"UPDATE bookmarks SET visit_count = visit_count + 1, last_visit = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339),
		bID,
	)
	if err != nil {
		return fmt.Errorf("failed to update visit count: %w", err)
	}

	return nil
}

func updateFavorite(tx *sqlx.Tx, b *bookmark.Bookmark) error {
	_, err := tx.ExecContext(
		context.Background(),
		"UPDATE bookmarks SET favorite = ? WHERE url = ?",
		b.Favorite,
		b.URL,
	)
	if err != nil {
		return fmt.Errorf("failed to update favorite: %w", err)
	}

	return nil
}
