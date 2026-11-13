package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"
)

type MetaKey string

const (
	// Default date format for timestamps.
	defaultDateFormat = "20060102-150405"

	// SQLite date format for timestamps.
	TimeFormatSqlite = "2006-01-02 15:04:05"

	// metadata keys.
	MetaKeyAppVersion MetaKey = "app_version"
	MetaKeyBackupAt   MetaKey = "backup_at"
	MetaKeyCreatedAt  MetaKey = "created_at"
)

func (r *SQLite) ReorderIDs(ctx context.Context) error {
	log := r.logger.With("fn", "db-reorder-ids", "db", r.Name())
	log.Debug("reordering bookmark IDs")

	bs, err := r.All(ctx)
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return err
	}
	if len(bs) == 0 {
		log.Debug("no bookmarks, nothing to reorder")
		return nil
	}

	mainTables := []Table{TableBookmarks, TableTags, TableRelation}

	err = r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, tbl := range mainTables {
			res, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tbl))
			if err != nil {
				return fmt.Errorf("clearing table %q: %w", tbl, err)
			}
			if n, err := res.RowsAffected(); err == nil {
				log.Debug("table cleared", "table", tbl, "rows", n)
			}
		}

		return r.resetSQLiteSequence(ctx, tx, mainTables...)
	})
	if err != nil {
		return err
	}

	// Reinsert bookmarks with new IDs
	log.Debug("reinserting bookmarks", "count", len(bs))
	if err := r.InsertMany(ctx, bs); err != nil {
		return err
	}

	log.Debug("reorder completed", "count", len(bs))

	return nil
}

// Backup creates a timestamped backup of the SQLite database at the specified destination.
// The backup filename follows the format: YYYYMMDD-HHMMSS_dbname.db.
func (r *SQLite) Backup(ctx context.Context, destRoot string) (string, error) {
	return r.newBackup(ctx, destRoot, time.Now())
}

func (r *SQLite) newBackup(ctx context.Context, destRoot string, now time.Time) (string, error) {
	if destRoot == "" {
		return "", ErrDBEmptyPath
	}

	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", now.Format(defaultDateFormat), r.Name())
	destPath := filepath.Join(destRoot, destDSN)

	log := r.logger.With("fn", "db-new-backup", "db", r.Name(), "dest", destPath)
	log.Debug("creating backup", "src", r.Cfg.Fullpath())

	_, err := os.Stat(destPath)
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}

	_ = r.DB.MustExecContext(ctx, "VACUUM INTO ?", destPath)
	log.Debug("vacuum into completed")

	backup, err := New(ctx, destPath)
	if err != nil {
		return "", err
	}
	defer backup.Close()

	if err := backup.SetBackupAt(ctx, now); err != nil {
		return "", err
	}

	if err := backup.CheckIntegrity(ctx); err != nil {
		return "", err
	}

	log.Debug("backup created")

	return destPath, nil
}

// CheckIntegrity performs a PRAGMA integrity_check on the SQLite database.
func (r *SQLite) CheckIntegrity(ctx context.Context) error {
	var result string
	row := r.DB.QueryRowContext(ctx, "PRAGMA integrity_check;")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrDBCorrupted, err)
	}

	if result != "ok" {
		r.logger.Warn("integrity check failed", "fn", "db-check-integrity", "db", r.Name(), "result", result)
		return fmt.Errorf("%w: integrity check: %q", ErrDBCorrupted, result)
	}

	r.logger.Debug("integrity verified", "fn", "db-check-integrity", "db", r.Name(), "result", result)

	return nil
}

func (r *SQLite) Metadata(key MetaKey) (string, error) {
	type metadata struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}

	var m metadata
	query := `SELECT key, value FROM metadata WHERE key = ?`
	err := r.DB.Get(&m, query, key)
	if err != nil {
		return "", err
	}
	return m.Value, nil
}

// SetMetadata sets or updates a metadata key.
func (r *SQLite) SetMetadata(ctx context.Context, key MetaKey, value string) error {
	const query = `
INSERT INTO metadata (key, value)
VALUES (?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value;`

	_, err := r.DB.ExecContext(ctx, query, key, value)
	return err
}

func (r *SQLite) UpdateAppVersion(ctx context.Context, version string) error {
	return r.SetMetadata(ctx, MetaKeyAppVersion, version)
}

func (r *SQLite) SetBackupAt(ctx context.Context, now time.Time) error {
	return r.SetMetadata(ctx, MetaKeyBackupAt, now.UTC().Format(TimeFormatSqlite))
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func (r *SQLite) resetSQLiteSequence(ctx context.Context, tx *sqlx.Tx, tables ...Table) error {
	log := r.logger.With("fn", "db-reset-sequence", "db", r.Name())

	if len(tables) == 0 {
		log.Warn("no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM sqlite_sequence WHERE name=?", t); err != nil {
			return fmt.Errorf("resetting sqlite sequence: %w", err)
		}
		log.Debug("sequence reset", "table", t)
	}

	return nil
}

// drop removes all records database.
func drop(ctx context.Context, r *SQLite) error {
	err := r.WithTx(ctx, func(tx *sqlx.Tx) error {
		if err := r.deleteAll(ctx, tx, tables...); err != nil {
			return err
		}

		return r.resetSQLiteSequence(ctx, tx, tables...)
	})
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.Vacuum(ctx)
}

func vacuum(ctx context.Context, r *SQLite) error {
	log := r.logger.With("fn", "db-vacuum", "db", r.Name())
	log.Debug("vacuuming database")

	_, err := r.DB.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	log.Debug("vacuum completed")

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
