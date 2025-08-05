// Package dbtask provides functions for managing SQLite databases.
package dbtask

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/pkg/db"
)

//nolint:unused //notimplementedyet
const defaultDateFormat = "20060102-150405"

var (
	ErrBackupExists   = errors.New("backup already exists")
	ErrDBCorrupted    = errors.New("database corrupted")
	ErrRecordNotFound = errors.New("no record found")
)

func Backup(fullpath string) (string, error) {
	r, err := db.New(fullpath)
	if err != nil {
		return "", err
	}

	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", time.Now().Format(r.Cfg.DateFormat), r.Name())
	destPath := filepath.Join(r.Cfg.BackupDir, destDSN)
	slog.Info("creating SQLite backup",
		"src", r.Cfg.Fullpath(),
		"dest", destPath,
	)

	if files.Exists(destPath) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}

	_ = r.DB.MustExec("VACUUM INTO ?", destPath)

	if err := db.VerifyIntegrity(destPath); err != nil {
		return "", err
	}

	return destPath, nil
}

// Drop removes all records database.
func Drop(ctx context.Context, r *db.SQLite) error {
	tables := []db.Table{
		db.Schemas.Main.Name,
		db.Schemas.Tags.Name,
		db.Schemas.Relation.Name,
	}

	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := deleteAll(ctx, r, tables...); err != nil {
			return err
		}
		return resetSQLiteSequence(ctx, tx, tables...)
	})
	if err != nil {
		return err
	}
	return Vacuum(ctx, r)
}

// DropOld removes all records database.
func DropOld(ctx context.Context, p string) error {
	r, err := db.New(p)
	if err != nil {
		return err
	}

	tables := []db.Table{
		db.Schemas.Main.Name,
		db.Schemas.Tags.Name,
		db.Schemas.Relation.Name,
	}

	err = r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := deleteAll(ctx, r, tables...); err != nil {
			return fmt.Errorf("%w", err)
		}

		return resetSQLiteSequence(ctx, tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return Vacuum(ctx, r)
}

// tableCreate creates a new table with the specified name in the SQLite database.
func tableCreate(ctx context.Context, tx *sqlx.Tx, name db.Table, schema string) error {
	slog.Debug("creating table", "name", name)

	_, err := tx.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	return nil
}

// tableExists checks whether a table with the specified name exists in the SQLite database.
func tableExists(r *db.SQLite, t db.Table) (bool, error) {
	var count int

	err := r.DB.Get(&count, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name = ?", t)
	if err != nil {
		slog.Error("checking if table exists", "name", t, "error", err)
		return false, fmt.Errorf("tableExists: %w", err)
	}

	return count > 0, nil
}

// tableDrop drops the specified table from the SQLite database.
func tableDrop(ctx context.Context, tx *sqlx.Tx, t db.Table) error {
	_, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return fmt.Errorf("%w: dropping table %q", err, t)
	}

	slog.Debug("dropped table", "name", t)

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func resetSQLiteSequence(ctx context.Context, tx *sqlx.Tx, tables ...db.Table) error {
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

// ReorderIDs reorders the IDs in the main table.
func ReorderIDs(ctx context.Context, r *db.SQLite) error {
	schemaMain := db.Schemas.Main
	schemaTemp := db.Schemas.Temp
	schemaRelation := db.Schemas.Relation

	return withTx(ctx, r, func(tx *sqlx.Tx) error {
		// check if last item has been deleted
		if r.MaxID() == 0 {
			return resetSQLiteSequence(ctx, tx, schemaMain.Name)
		}
		// get all records
		bs, err := r.All()
		if err != nil {
			if !errors.Is(ErrRecordNotFound, err) {
				return err
			}
		}

		if len(bs) == 0 {
			return nil
		}
		// drop the trigger to avoid errors during the table reorganization.
		if _, err := tx.ExecContext(ctx, "DROP TRIGGER IF EXISTS cleanup_bookmark_and_tags"); err != nil {
			return fmt.Errorf("dropping trigger: %w", err)
		}
		// create temp table
		if err := tableCreate(ctx, tx, schemaTemp.Name, schemaTemp.SQL); err != nil {
			return err
		}
		// populate temp table
		if err := insertManyIntoTempTable(ctx, tx, bs); err != nil {
			return fmt.Errorf("%w: insert many (table %q)", err, schemaTemp.Name)
		}
		// drop main table
		if err := tableDrop(ctx, tx, schemaMain.Name); err != nil {
			return err
		}
		// rename temp table to main table
		if err := tableRename(ctx, tx, schemaTemp.Name, schemaMain.Name); err != nil {
			return err
		}
		// create index
		if _, err := tx.ExecContext(ctx, schemaTemp.Index); err != nil {
			return fmt.Errorf("creating index: %w", err)
		}
		// restore relational table trigger
		for _, t := range schemaRelation.Trigger {
			if _, err := tx.ExecContext(ctx, t); err != nil {
				return fmt.Errorf("restoring trigger: %w", err)
			}
		}

		return nil
	})
}

// insertManyIntoTempTable inserts multiple records into a temporary table.
func insertManyIntoTempTable(ctx context.Context, tx *sqlx.Tx, bs []*db.BookmarkModel) error {
	q := `
  INSERT INTO temp_bookmarks (
    url, title, desc, created_at, last_visit,
    updated_at, visit_count, favorite, checksum, favicon_url
  )
  VALUES
    (
      :url, :title, :desc, :created_at, :last_visit,
      :updated_at, :visit_count, :favorite, :checksum, :favicon_url
    )
  `

	stmt, err := tx.PrepareNamedContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare statement: %w", err)
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Error("delete many: closing stmt", "error", err)
		}
	}()

	for _, b := range bs {
		if _, err := stmt.Exec(b); err != nil {
			return fmt.Errorf("insert bookmark %s: %w", b.URL, err)
		}
	}

	return nil
}

// tableRename renames the temporary table to the specified main table name.
func tableRename(ctx context.Context, tx *sqlx.Tx, srcTable, destTable db.Table) error {
	slog.Debug("renaming table", "from", srcTable, "to", destTable)

	_, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", srcTable, destTable))
	if err != nil {
		return fmt.Errorf("%w: renaming table from %q to %q", err, srcTable, destTable)
	}

	return nil
}

// withTx executes a function within a transaction.
func withTx(ctx context.Context, r *db.SQLite, fn func(tx *sqlx.Tx) error) error {
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

// deleteAll deletes all records in the give table.
func deleteAll(ctx context.Context, r *db.SQLite, ts ...db.Table) error {
	if len(ts) == 0 {
		slog.Debug("no tables to delete")
		return nil
	}

	slog.Debug("deleting all records from tables", "tables", ts)

	return withTx(ctx, r, func(tx *sqlx.Tx) error {
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

func Vacuum(ctx context.Context, r *db.SQLite) error {
	slog.Debug("vacuuming database")

	_, err := r.DB.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// IsInitialized returns true if the database is initialized.
func IsInitialized(r *db.SQLite) bool {
	allExist := true

	schemas := []db.Schema{
		db.Schemas.Main, db.Schemas.Tags, db.Schemas.Relation,
	}

	for _, s := range schemas {
		exists, err := tableExists(r, s.Name)
		if err != nil {
			slog.Error("checking if table exists", "name", s.Name, "error", err)
			return false
		}

		if !exists {
			allExist = false

			slog.Warn("table does not exist", "name", s.Name)
		}
	}

	return allExist
}

// Init initializes a new database and creates the required tables.
func Init(ctx context.Context, r *db.SQLite) error {
	schemas := []db.Schema{
		db.Schemas.Main,
		db.Schemas.Tags,
		db.Schemas.Relation,
	}

	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, s := range schemas {
			if err := tableCreate(ctx, tx, s.Name, s.SQL); err != nil {
				return fmt.Errorf("creating %q table: %w", s.Name, err)
			}

			if s.Index != "" {
				if _, err := tx.ExecContext(ctx, s.Index); err != nil {
					return fmt.Errorf("creating %q index: %w", s.Name, err)
				}
			}

			if len(s.Trigger) > 0 {
				for _, t := range s.Trigger {
					if _, err := tx.ExecContext(ctx, t); err != nil {
						return fmt.Errorf("creating %q trigger: %w", s.Name, err)
					}
				}
			}
		}

		return nil
	})
}
