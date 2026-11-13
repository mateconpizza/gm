package db

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/pkg/bookmark"
)

const bookmarkColumns = `
	b.id,
	b.url,
	b.title,
	b.desc,
	b.notes,
	b.created_at,
	b.updated_at,
	b.last_visit,
	b.last_checked,
	b.visit_count,
	b.favorite,
	b.status_code,
	b.status_text,
	b.is_active,
	b.favicon_url,
	b.favicon_local,
	b.archive_url,
	b.archive_timestamp,
	b.checksum
`

// InsertOne creates a new record in the main table.
func (r *SQLite) InsertOne(ctx context.Context, b *bookmark.Bookmark) (int64, error) {
	log := r.logger.With("fn", "db-insert-one", "db", r.Name())

	if err := bookmark.Validate(b); err != nil {
		log.Debug("validation failed", "url", b.URL, "error", err)
		return 0, err
	}

	var id int64
	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		var err error
		id, err = r.insertIntoTx(ctx, tx, b)
		return err
	})
	if err != nil {
		return 0, err
	}

	log.Debug("bookmark inserted", "id", id, "url", b.URL)

	return id, nil
}

func (r *SQLite) InsertMany(ctx context.Context, bs []*bookmark.Bookmark) error {
	return r.insertBulkPtr(ctx, bs)
}

// DeleteMany deletes multiple records from the main table.
func (r *SQLite) DeleteMany(ctx context.Context, ids []int) error {
	n := len(ids)
	if n == 0 {
		return ErrRecordIDNotProvided
	}

	log := r.logger.With("fn", "db-delete-many", "db", r.Name())
	log.Debug("deleting bookmarks", "count", n, "ids", ids)

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Delete from bookmark_tags first (foreign key constraint)
		q1, args1, err := sqlx.In("DELETE FROM bookmark_tags WHERE bookmark_id IN (?)", ids)
		if err != nil {
			return fmt.Errorf("preparing bookmark_tags delete: %w", err)
		}
		_, err = tx.ExecContext(ctx, q1, args1...)
		if err != nil {
			return fmt.Errorf("deleting from bookmark_tags: %w", err)
		}

		// Delete from bookmarks table
		q2, args2, err := sqlx.In("DELETE FROM bookmarks WHERE id IN (?)", ids)
		if err != nil {
			return fmt.Errorf("preparing bookmarks delete: %w", err)
		}
		res, err := tx.ExecContext(ctx, q2, args2...)
		if err != nil {
			return fmt.Errorf("deleting from bookmarks: %w", err)
		}
		if affected, err := res.RowsAffected(); err == nil {
			log.Debug("bookmarks deleted", "requested", n, "affected", affected)
		}

		// Clean up orphaned tags
		return r.cleanOrphanTagsTx(ctx, tx)
	})
	if err != nil {
		return err
	}

	log.Debug("delete committed", "count", n)

	return nil
}

// UpdateOne updates an existing bookmark by ID (or URL).
func (r *SQLite) UpdateOne(ctx context.Context, b *bookmark.Bookmark) error {
	log := r.logger.With("fn", "db-update-one", "db", r.Name(), "id", b.ID, "url", b.URL)
	log.Debug("updating bookmark")

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Update record
		if err := r.updateRecordTx(ctx, tx, b); err != nil {
			return fmt.Errorf("update record: %w", err)
		}

		// Remove old tag associations
		if _, err := tx.ExecContext(ctx, "DELETE FROM bookmark_tags WHERE bookmark_id = ?", b.ID); err != nil {
			return fmt.Errorf("clear tags: %w", err)
		}

		// Re-associate tags
		if err := r.associateTags(ctx, tx, b); err != nil {
			return fmt.Errorf("associate tags: %w", err)
		}

		return r.cleanOrphanTagsTx(ctx, tx)
	})
	if err != nil {
		return err
	}

	log.Debug("bookmark updated")

	return nil
}

// updateRecordTx updates a bookmark inside a transaction.
func (r *SQLite) updateRecordTx(ctx context.Context, tx *sqlx.Tx, b *bookmark.Bookmark) error {
	b.GenChecksum()
	b.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	query := `
	UPDATE bookmarks
	SET
		url = :url,
		title = :title,
		desc = :desc,
		notes = :notes,
		last_visit = :last_visit,
		updated_at = :updated_at,
		visit_count = :visit_count,
		favorite = :favorite,
		checksum = :checksum,
		favicon_url = :favicon_url,
		favicon_local = :favicon_local,
		archive_url = :archive_url,
		archive_timestamp = :archive_timestamp,
		last_checked = :last_checked,
		status_code = :status_code,
		status_text = :status_text,
		is_active = :is_active
	WHERE id = :id OR url = :url
	`

	res, err := tx.NamedExecContext(ctx, query, b)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if n, err := res.RowsAffected(); err == nil {
		r.logger.Debug("bookmark record updated", "fn", "db-update-record", "id", b.ID, "rows", n)
	}

	return nil
}

// All returns all bookmarks.
func (r *SQLite) All(ctx context.Context) ([]*bookmark.Bookmark, error) {
	log := r.logger.With("fn", "db-all", "db", r.Name())
	log.Debug("fetching all bookmarks")

	q := `
    SELECT
			` + bookmarkColumns + `,
      COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
      LEFT JOIN tags t ON bt.tag_id = t.id
    GROUP BY
      b.id
    ORDER BY
      b.id ASC;`

	bs, err := r.bySQL(ctx, q)
	if err != nil {
		return nil, err
	}

	log.Debug("fetched bookmarks", "count", len(bs))

	return bs, nil
}

// ByID returns a record by its ID in the give table.
func (r *SQLite) ByID(ctx context.Context, bID int) (*bookmark.Bookmark, error) {
	log := r.logger.With("fn", "db-by-id", "db", r.Name(), "id", bID)

	maxID := r.MaxID(ctx)
	if bID > maxID {
		log.Debug("id out of range", "max", maxID)
		return nil, fmt.Errorf("%w. max: %d", ErrRecordNotFound, maxID)
	}

	log.Debug("getting record by ID")

	q := `
    SELECT
			` + bookmarkColumns + `,
      COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
      LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
      b.id = ?`

	var b bookmark.Bookmark
	err := r.DB.GetContext(ctx, &b, q, bID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Debug("record not found")
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
			`+bookmarkColumns+`,
      COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
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
			`+bookmarkColumns+`,
      COALESCE(GROUP_CONCAT(t.name, ','), '') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
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
	query := `
		SELECT
			` + bookmarkColumns + `,
			GROUP_CONCAT(t2.name, ',') AS tags
		FROM bookmarks b
		JOIN bookmark_tags bt ON b.id = bt.bookmark_id
		JOIN tags t ON t.id = bt.tag_id

		LEFT JOIN bookmark_tags bt2 ON b.id = bt2.bookmark_id
		LEFT JOIN tags t2 ON t2.id = bt2.tag_id

		WHERE t.name = ?
		GROUP BY b.id
		ORDER BY b.id ASC;
	`

	bs, err := r.bySQL(ctx, query, tag)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// ByQuery returns records by query in the give table.
func (r *SQLite) ByQuery(ctx context.Context, query string) ([]*bookmark.Bookmark, error) {
	log := r.logger.With("fn", "db-by-query", "db", r.Name(), "query", query)
	log.Debug("getting records by query")

	q := `
    SELECT
			` + bookmarkColumns + `,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM bookmarks b
		LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
    LEFT JOIN tags t ON bt.tag_id = t.id
    WHERE
        (LOWER(b.id || b.title || b.url || b.desc || b.notes) LIKE LOWER(?) OR
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
		log.Debug("no matches")
		return nil, ErrRecordNoMatch
	}

	log.Debug("got records by query", "count", len(bs))

	return bs, nil
}

// MaxID retrieves the maximum ID from the specified table in the SQLite
// database.
func (r *SQLite) MaxID(ctx context.Context) int {
	var lastIndex int
	if err := r.DB.QueryRowxContext(ctx, "SELECT COALESCE(MAX(id), 0) FROM bookmarks").Scan(&lastIndex); err != nil {
		r.logger.Debug("max id failed", "fn", "db-max-id", "error", err)
		return 0
	}

	return lastIndex
}

// WithTx executes a function within a transaction.
func (r *SQLite) WithTx(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	log := r.logger.With("fn", "db-with-tx", "db", r.Name())

	if err := ctx.Err(); err != nil {
		log.Debug("context done before transaction", "cause", context.Cause(ctx))
		return err
	}

	tx, err := r.DB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Handle rollback only on failure.
	defer func() {
		if p := recover(); p != nil {
			log.Error("panic in transaction, rolling back", "panic", p)
			_ = tx.Rollback() // ensure rollback on panic
			panic(p)          // re-throw the panic
		} else if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				log.Debug("transaction rollback failed",
					"original_error", err,
					"rollback_error", rollbackErr,
				)
				return
			}
			log.Debug("transaction rolled back", "error", err)
		}
	}()

	// Assign to the named return value 'err'.
	if err = fn(tx); err != nil {
		return fmt.Errorf("fn transaction: %w", err)
	}

	// Assign to the named return value 'err'.
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	log.Debug("transaction committed")

	return nil
}

func (r *SQLite) AddVisit(ctx context.Context, bID int) error {
	log := r.logger.With("fn", "db-add-visit", "db", r.Name(), "id", bID)

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		return updateVisit(ctx, tx, bID)
	})
	if err != nil {
		return err
	}

	log.Debug("visit added")

	return nil
}

func (r *SQLite) SetFavorite(ctx context.Context, b *bookmark.Bookmark) error {
	log := r.logger.With("fn", "db-set-favorite", "db", r.Name(), "id", b.ID)

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		return updateFavorite(ctx, tx, b)
	})
	if err != nil {
		return err
	}

	log.Debug("favorite updated", "favorite", b.Favorite)

	return nil
}

// FavoritesList returns the favorite bookmarks.
func (r *SQLite) FavoritesList(ctx context.Context) ([]*bookmark.Bookmark, error) {
	q := `
    SELECT
			` + bookmarkColumns + `,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
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
			`+bookmarkColumns+`,
      GROUP_CONCAT(t.name, ',') AS tags
    FROM
      bookmarks b
      LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
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
func (r *SQLite) Count(ctx context.Context, table Table) int {
	var n int
	if err := r.DB.QueryRowxContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&n); err != nil {
		r.logger.Debug("count failed", "fn", "db-count", "table", table, "error", err)
		return 0
	}

	return n
}

// Has checks if a record exists in the main table.
func (r *SQLite) Has(ctx context.Context, bURL string) (*bookmark.Bookmark, bool) {
	log := r.logger.With("fn", "db-has", "db", r.Name(), "url", bURL)

	var exists bool
	err := r.DB.QueryRowxContext(ctx, "SELECT EXISTS(SELECT 1 FROM bookmarks WHERE url = ?)", bURL).
		Scan(&exists)
	if err != nil {
		log.Debug("error checking existence", "error", err)
		return nil, false
	}

	if !exists {
		log.Debug("not found")
		return nil, false
	}

	item, err := r.ByURL(ctx, bURL)
	if err != nil {
		log.Debug("exists but failed to load", "error", err)
		return nil, false
	}

	log.Debug("found", "id", item.ID)

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
func (r *SQLite) deleteOneTx(ctx context.Context, tx *sqlx.Tx, b *bookmark.Bookmark) error {
	log := r.logger.With("fn", "db-delete-one-tx", "id", b.ID, "url", b.URL)
	log.Debug("deleting record")

	// remove tags relationships first
	if _, err := tx.ExecContext(
		ctx,
		fmt.Sprintf("DELETE FROM %s WHERE bookmark_id = ?", TableRelation.String()),
		b.ID,
	); err != nil {
		return fmt.Errorf("failed to delete tags: %w", err)
	}

	// remove main record
	res, err := tx.ExecContext(ctx, "DELETE FROM bookmarks WHERE id = ?", b.ID)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	if n, err := res.RowsAffected(); err == nil {
		log.Debug("record deleted", "rows", n)
	}

	return nil
}

// deleteAll deletes all records in the give table.
func (r *SQLite) deleteAll(ctx context.Context, tx *sqlx.Tx, ts ...Table) error {
	log := r.logger.With("fn", "db-delete-all", "db", r.Name())

	if len(ts) == 0 {
		log.Debug("no tables to delete")
		return nil
	}

	log.Debug("deleting all records", "tables", ts)

	for _, t := range ts {
		res, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", t))
		if err != nil {
			return fmt.Errorf("failed to delete from %s: %w", t, err)
		}

		if n, err := res.RowsAffected(); err == nil {
			log.Debug("table cleared", "table", t, "rows", n)
		}
	}

	return nil
}

func (r *SQLite) insertBulkPtr(ctx context.Context, bs []*bookmark.Bookmark) error {
	log := r.logger.With("fn", "db-insert-bulk", "db", r.Name())
	log.Debug("inserting records", "count", len(bs))

	sort.Slice(bs, func(i, j int) bool {
		return bs[i].ID < bs[j].ID
	})

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, b := range bs {
			if _, err := r.insertIntoTx(ctx, tx, b); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Debug("bulk insert committed", "count", len(bs))

	return nil
}

// insertIntoTx inserts a record inside an existing transaction.
func (r *SQLite) insertIntoTx(ctx context.Context, tx *sqlx.Tx, b *bookmark.Bookmark) (int64, error) {
	// insert record and associate tags in the same transaction.
	bID, err := insertRecord(ctx, tx, b)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", err, b.URL)
	}

	if err := r.associateTags(ctx, tx, b); err != nil {
		return 0, fmt.Errorf("failed to associate tags: %w", err)
	}

	r.logger.Debug("record inserted in tx", "fn", "db-insert-into-tx", "id", bID, "url", b.URL)

	return bID, nil
}

// insertRecord inserts a new record into the table.
func insertRecord(ctx context.Context, tx *sqlx.Tx, b *bookmark.Bookmark) (int64, error) {
	b.GenChecksum()
	b.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	r, err := tx.NamedExecContext(
		ctx,
		`INSERT INTO bookmarks (
			url,
			title,
			desc,
			notes,
			created_at,
			last_visit,
			updated_at,
			visit_count,
			favorite,
			checksum,
			favicon_url,
			favicon_local,
			archive_url,
			archive_timestamp,
			last_checked,
			status_code,
			status_text
		)
		VALUES (
			:url,
			:title,
			:desc,
			:notes,
			:created_at,
			:last_visit,
			:updated_at,
			:visit_count,
			:favorite,
			:checksum,
			:favicon_url,
			:favicon_local,
			:archive_url,
			:archive_timestamp,
			:last_checked,
			:status_code,
			:status_text
	)`, b,
	)
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
func updateVisit(ctx context.Context, tx *sqlx.Tx, bID int) error {
	slog.Debug("updating visit count", "id", bID)
	_, err := tx.ExecContext(
		ctx,
		"UPDATE bookmarks SET visit_count = visit_count + 1, last_visit = ? WHERE id = ?",
		time.Now().UTC().Format(time.RFC3339),
		bID,
	)
	if err != nil {
		return fmt.Errorf("failed to update visit count: %w", err)
	}

	return nil
}

func updateFavorite(ctx context.Context, tx *sqlx.Tx, b *bookmark.Bookmark) error {
	_, err := tx.ExecContext(
		ctx,
		"UPDATE bookmarks SET favorite = ? WHERE id = ?",
		b.Favorite,
		b.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update favorite: %w", err)
	}

	return nil
}
