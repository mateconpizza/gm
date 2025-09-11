package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
)

// IsInitialized checks if the database is initialized.
func IsInitialized(p string) (bool, error) {
	slog.Debug("checking if database is initialized", "path", p)

	allExist := true

	r, err := New(p)
	if err != nil {
		return false, err
	}

	for _, s := range tablesAndSchemas {
		exists, err := tableExists(r, s.Name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.Name, "error", err)
			return false, err
		}

		if !exists {
			allExist = false

			slog.Warn("table does not exist", "name", s.Name)
		}
	}

	return allExist, nil
}

// drop removes all records database.
func drop(ctx context.Context, r *SQLite) error {
	tables := make([]Table, 0, len(tablesAndSchemas))
	for _, t := range tablesAndSchemas {
		tables = append(tables, t.Name)
	}

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteAll(ctx, tx, tables...); err != nil {
			return err
		}

		return resetSQLiteSequence(ctx, tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.Vacuum(ctx)
}

func vacuum(ctx context.Context, r *SQLite) error {
	slog.Debug("vacuuming database")

	_, err := r.DB.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func resetSQLiteSequence(ctx context.Context, tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		slog.Warn("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		slog.Debug("resetting sqlite sequence", "table", t)

		if _, err := tx.ExecContext(ctx, "DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
	}

	return nil
}

// fileExists checks if a file exists.
func fileExists(s string) bool {
	_, err := os.Stat(s)
	return !os.IsNotExist(err)
}

func ensureDBSuffix(s string) string {
	const suffix = ".db"
	if s == "" {
		return s
	}

	e := filepath.Ext(s)
	if e == suffix || e != "" {
		return s
	}

	return fmt.Sprintf("%s%s", s, suffix)
}

// CleanupOrphanTags elimina todos los tags que no estén asociados a ningún bookmark.
func (r *SQLite) CleanupOrphanTags(ctx context.Context) error {
	_, err := r.DB.ExecContext(ctx, `
		DELETE FROM tags
		WHERE id NOT IN (
			SELECT DISTINCT tag_id FROM bookmark_tags
		);
	`)
	return err
}

func (r *SQLite) cleanOrphanTagsTx(tx *sqlx.Tx) error {
	_, err := tx.Exec(`
		DELETE FROM tags
		WHERE id NOT IN (
			SELECT DISTINCT tag_id FROM bookmark_tags
		);`)
	if err != nil {
		return err
	}

	return nil
}

func (r *SQLite) ReorderIDs(ctx context.Context) error {
	slog.Debug("Reordering bookmark IDs")

	// Get all bookmarks in memory
	bs, err := r.All(ctx)
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return err
	}
	if len(bs) == 0 {
		return nil
	}

	err = r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, tbl := range []Table{schemaMain.Name, schemaTags.Name, schemaRelation.Name} {
			slog.Debug("Deleting records from", "table", tbl)
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tbl)); err != nil {
				return fmt.Errorf("clearing table %q: %w", tbl, err)
			}
		}

		return resetSQLiteSequence(ctx, tx, schemaMain.Name, schemaTags.Name, schemaRelation.Name)
	})
	if err != nil {
		return err
	}

	// Reinsert bookmarks with new IDs
	return r.InsertMany(ctx, bs)
}
