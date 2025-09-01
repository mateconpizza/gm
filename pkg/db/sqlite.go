// Package db provides the model of the configuration for a database.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const (
	MaxOpenConns    = 10        // Maximum number of open connections
	MaxIdleConns    = 5         // Maximum number of idle connections
	MaxLifetimeConn = time.Hour // Maximum connection lifetime
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

// buildSQLiteDSN constructs a SQLite Data Source Name from a file path and
// optional parameters.
func buildSQLiteDSN(path string, params map[string]string) string {
	queryParams := url.Values{}
	for key, value := range params {
		queryParams.Add(key, value)
	}

	if len(queryParams) == 0 {
		return path
	}

	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return fmt.Sprintf("%s%s%s", path, separator, queryParams.Encode())
}

// OpenDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func OpenDatabase(path string) (*sqlx.DB, error) {
	slog.Debug("opening database", "path", path)
	isTestingMode := strings.Contains(path, "mode=memory") || path == ":memory:"

	dbParams := map[string]string{
		"_journal_mode": "WAL",    // enable multi-thread safe mode with wal
		"_foreign_keys": "on",     // enforce foreign key constraints
		"_synchronous":  "NORMAL", // balance performance and durability
		"_busy_timeout": "5000",   // set a timeout for a busy database
	}

	dsn := buildSQLiteDSN(path, dbParams)
	// testing mode
	if isTestingMode {
		dsn = buildSQLiteDSN(path, map[string]string{"_foreign_keys": "on"})
	}

	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Connection pool tuning
	db.SetMaxOpenConns(MaxOpenConns)
	db.SetMaxIdleConns(MaxIdleConns)
	db.SetConnMaxLifetime(MaxLifetimeConn)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%w: on ping context", err)
	}

	return db, nil
}

// Cfg represents the configuration for a SQLite database.
type Cfg struct {
	Name string `json:"name"` // Name of the SQLite database
	Path string `json:"path"` // Path to the SQLite database
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
