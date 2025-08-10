// Package db provides the model of the configuration for a database.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Table string

// SQLite implements the Repository interface.
type SQLite struct {
	DB        *sqlx.DB `json:"-"`
	Cfg       *Cfg     `json:"db"`
	closeOnce sync.Once
}

// Name returns the name of the SQLite database.
func (r *SQLite) Name() string {
	return r.Cfg.Name
}

// Close closes the SQLite database connection and logs any errors encountered.
func (r *SQLite) Close() {
	s := r.Name()
	r.closeOnce.Do(func() {
		if err := r.DB.Close(); err != nil {
			slog.Error("closing database", "name", s, "error", err)
		} else {
			slog.Debug("database closed", "name", s)
		}
	})
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sqlx.DB, cfg *Cfg) *SQLite {
	return &SQLite{
		DB:  db,
		Cfg: cfg,
	}
}

// New returns a new SQLiteRepository from an existing database path.
func New(p string) (*SQLite, error) {
	return newRepository(p, func(path string) error {
		slog.Debug("new repo: checking if database exists")

		if !fileExists(path) {
			return fmt.Errorf("%w: %q", ErrDBNotFound, path)
		}

		return nil
	})
}

// Init initializes a new SQLiteRepository at the provided path.
func Init(p string) (*SQLite, error) {
	return newRepository(p, func(path string) error {
		slog.Debug("init repo: checking if database exists", "path", path)

		if fileExists(path) {
			return fmt.Errorf("%w: %q", ErrDBExists, path)
		}

		return nil
	})
}

// newRepository returns a new SQLiteRepository from the provided path.
func newRepository(p string, validate func(string) error) (*SQLite, error) {
	if p == "" {
		return nil, fmt.Errorf("%w: %q", ErrDBNotFound, p)
	}

	if err := validate(p); err != nil {
		return nil, err
	}

	c, err := NewSQLiteCfg(p)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	db, err := OpenDatabase(p)
	if err != nil {
		slog.Error("NewRepo", "error", err, "path", p)
		return nil, err
	}

	return newSQLiteRepository(db, c), nil
}

// NewFromBackup creates a SQLiteRepository from a backup file.
func NewFromBackup(backupPath string) (*SQLite, error) {
	if backupPath == "" {
		return nil, fmt.Errorf("%w: %q", ErrDBNotFound, backupPath)
	}

	var (
		backupFile = filepath.Base(backupPath)
		parentDir  = filepath.Dir(backupPath)
	)

	// check if we're already in a backup directory
	if filepath.Base(parentDir) == "backup" {
		parentDir = filepath.Dir(filepath.Dir(backupPath))
	} else {
		parentDir = filepath.Dir(backupPath)
	}

	backupDir := filepath.Join(parentDir, "backup")
	cfg := &Cfg{
		Path: backupDir,
		Name: backupFile,
	}

	slog.Debug("reading backup", "name", backupFile)

	db, err := OpenDatabase(backupPath)
	if err != nil {
		return nil, fmt.Errorf("opening backup database: %w", err)
	}

	return &SQLite{
		DB:  db,
		Cfg: cfg,
	}, nil
}

// OpenDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func OpenDatabase(s string) (*sqlx.DB, error) {
	slog.Debug("opening database", "path", s)

	db, err := sqlx.Open("sqlite3", s)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%w: on ping context", err)
	}

	return db, nil
}

// Cfg represents the configuration for a SQLite database.
type Cfg struct {
	Name        string   `json:"name"`         // Name of the SQLite database
	Path        string   `json:"path"`         // Path to the SQLite database
	BackupFiles []string `json:"backup_files"` // Backup files
}

// Fullpath returns the full path to the SQLite database.
func (c *Cfg) Fullpath() string {
	return filepath.Join(c.Path, c.Name)
}

// Exists returns true if the SQLite database exists.
func (c *Cfg) Exists() bool {
	return fileExists(c.Fullpath())
}

// NewSQLiteCfg returns the default settings for the database.
func NewSQLiteCfg(p string) (*Cfg, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %q: %w", p, err)
	}

	baseDir := filepath.Dir(abs)

	return &Cfg{
		Path: baseDir,
		Name: ensureDBSuffix(filepath.Base(abs)),
	}, nil
}
