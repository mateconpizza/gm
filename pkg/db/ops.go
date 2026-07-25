package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	slog.DebugContext(ctx, "Reordering bookmark IDs")

	bs, err := r.All(ctx)
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return err
	}
	if len(bs) == 0 {
		return nil
	}

	mainTables := []Table{TableBookmarks, TableTags, TableRelation}

	err = r.WithTx(ctx, func(tx *sqlx.Tx) error {
		for _, tbl := range mainTables {
			slog.DebugContext(ctx, "deleting records from", "table", tbl)
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", tbl)); err != nil {
				return fmt.Errorf("clearing table %q: %w", tbl, err)
			}
		}

		return resetSQLiteSequence(ctx, tx, mainTables...)
	})
	if err != nil {
		return err
	}

	// Reinsert bookmarks with new IDs
	return r.InsertMany(ctx, bs)
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
	slog.InfoContext(ctx, "creating SQLite backup", "src", r.Cfg.Fullpath(), "dest", destPath)

	_, err := os.Stat(destPath)
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}

	_ = r.DB.MustExecContext(ctx, "VACUUM INTO ?", destPath)

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
		return fmt.Errorf("%w: integrity check: %q", ErrDBCorrupted, result)
	}

	slog.DebugContext(ctx, "SQLite integrity verified", "result", result)

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

// IsInitializedFromPath checks if the database is initialized.
func IsInitializedFromPath(ctx context.Context, p string) (bool, error) {
	slog.DebugContext(ctx, "checking if database is initialized", "path", p)
	r, err := New(ctx, p)
	if err != nil {
		return false, err
	}
	defer r.Close()

	return r.IsInitialized(ctx), nil
}

// drop removes all records database.
func drop(ctx context.Context, r *SQLite) error {
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
	slog.DebugContext(ctx, "vacuuming database")

	_, err := r.DB.ExecContext(ctx, "VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

// resetSQLiteSequence resets the SQLite sequence for the given table.
func resetSQLiteSequence(ctx context.Context, tx *sqlx.Tx, tables ...Table) error {
	if len(tables) == 0 {
		slog.WarnContext(ctx, "no tables provided to reset sqlite sequence")
		return nil
	}

	for _, t := range tables {
		slog.DebugContext(ctx, "resetting sqlite sequence", "table", t)

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
