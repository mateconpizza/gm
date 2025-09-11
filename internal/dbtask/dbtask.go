// Package dbtask provides functions for managing SQLite databases.
package dbtask

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// Default date format for timestamps.
const defaultDateFormat = "20060102-150405"

var (
	ErrBackupExists   = errors.New("backup already exists")
	ErrDBCorrupted    = errors.New("database corrupted")
	ErrRecordNotFound = errors.New("no record found")
)

// RepoStats holds statistics about a bookmark repository.
type RepoStats struct {
	Name      string `json:"dbname"`    // Name is the database base name.
	Bookmarks int    `json:"bookmarks"` // Bookmarks is the count of bookmarks.
	Tags      int    `json:"tags"`      // Tags is the count of tags.
	Favorites int    `json:"favorites"` // Favorites is the count of favorite bookmarks.
	Size      string `json:"size"`
}

func Backup(fullpath string) (string, error) {
	r, err := db.New(fullpath)
	if err != nil {
		return "", err
	}

	// destDSN -> 20060102-150405_dbName.db
	destDSN := fmt.Sprintf("%s_%s", time.Now().Format(defaultDateFormat), r.Name())
	destPath := filepath.Join(config.App.Path.Backup, destDSN)
	slog.Info("creating SQLite backup",
		"src", r.Cfg.Fullpath(),
		"dest", destPath,
	)

	if files.Exists(destPath) {
		return "", fmt.Errorf("%w: %q", ErrBackupExists, destPath)
	}

	_ = r.DB.MustExec("VACUUM INTO ?", destPath)

	if err := VerifyIntegrity(destPath); err != nil {
		return "", err
	}

	return destPath, nil
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
		if _, err := tx.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("enabling foreign keys: %w", err)
		}

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

// DropFromPath drops the database from the given path.
func DropFromPath(dbPath string) error {
	r, err := db.New(dbPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return r.DropSecure(context.Background())
}

// VerifyIntegrity checks the integrity of the SQLite database.
func VerifyIntegrity(path string) error {
	slog.Debug("verifying SQLite integrity", "path", path)

	r, err := db.OpenDatabase(path)
	if err != nil {
		return fmt.Errorf("no se pudo abrir backup: %w", err)
	}

	defer func() {
		if err := r.Close(); err != nil {
			slog.Error("error closing db", "error", err)
		}
	}()

	var result string
	ctx := context.Background()
	row := r.QueryRowContext(ctx, "PRAGMA integrity_check;")
	if err := row.Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrDBCorrupted, err)
	}

	if result != "ok" {
		return fmt.Errorf("%w: integrity check: %q", ErrDBCorrupted, result)
	}

	slog.Debug("SQLite integrity verified", "result", result)

	return nil
}

// TagsCounterFromPath returns a map with tag as key and count as value.
func TagsCounterFromPath(dbPath string) (map[string]int, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return r.TagsCounter(context.Background())
}

func NewRepoStats(dbPath string) (*RepoStats, error) {
	r, err := db.New(dbPath)
	if err != nil {
		return nil, fmt.Errorf("creating repo: %w", err)
	}
	defer r.Close()

	ctx := context.Background()

	return &RepoStats{
		Name:      r.Name(),
		Bookmarks: r.Count(ctx, "bookmarks"),
		Tags:      r.Count(ctx, "tags"),
		Favorites: r.CountFavorites(ctx),
		Size:      files.SizeFormatted(r.Cfg.Fullpath()),
	}, nil
}
