package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const (
	_maxBytesSize int64  = 300000
	_mainTable    string = "bookmarks"
	_deletedTable string = "deleted_bookmarks"
	_dateFormat   string = "2006-01-02 15:04:05"
)

// SQLiteRepository implements the Repository interface
type SQLiteRepository struct {
	DB  *sql.DB       `json:"-"`
	Cfg *SQLiteConfig `json:"db"`
}

func (r *SQLiteRepository) Close() {
	if err := r.DB.Close(); err != nil {
		log.Printf("closing database: %v", err)
	}
}

// newSQLiteRepository returns a new SQLiteRepository
func newSQLiteRepository(db *sql.DB, cfg *SQLiteConfig) *SQLiteRepository {
	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}
}

// New returns a new SQLiteRepository
func New(c *SQLiteConfig) (*SQLiteRepository, error) {
	name := c.GetName()
	if !strings.HasSuffix(name, ".db") {
		name = fmt.Sprintf("%s.db", name)
	}

	db, err := MustOpenDatabase(filepath.Join(c.GetHome(), name))
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.maintenance(c); err != nil {
		return nil, err
	}
	return r, nil
}

// MustOpenDatabase opens a database
func MustOpenDatabase(path string) (*sql.DB, error) {
	log.Printf("opening database: '%s'", path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		panic(fmt.Errorf("%w: on ping context", err))
	}

	return db, nil
}

// SQLIteConfig is the configuration for the SQLite database
type SQLiteConfig struct {
	Name         string  `json:"name"`
	Home         string  `json:"path"`
	Backup       *Backup `json:"backup"`
	Type         string  `json:"type"`
	TableMain    string  `json:"table_main"`
	TableDeleted string  `json:"table_deleted"`
	MaxBytesSize int64   `json:"max_bytes_size"`
}

func (c *SQLiteConfig) GetHome() string {
	return c.Home
}

func (c *SQLiteConfig) GetName() string {
	return c.Name
}

func (c *SQLiteConfig) Fullpath() string {
	return filepath.Join(c.GetHome(), c.GetName())
}

func (c *SQLiteConfig) GetMaxSizeBytes() int64 {
	return c.MaxBytesSize
}

func (c *SQLiteConfig) GetTableMain() string {
	return c.TableMain
}

func (c *SQLiteConfig) GetTableDeleted() string {
	return c.TableDeleted
}

func (c *SQLiteConfig) SetHome(path string) {
	c.Home = path
	c.Backup = newBackup(path)
}

func (c *SQLiteConfig) SetName(name string) {
	c.Name = name
}

// NewSQLiteCfg returns the default settings for the database
func NewSQLiteCfg() *SQLiteConfig {
	return &SQLiteConfig{
		TableMain:    _mainTable,
		TableDeleted: _deletedTable,
		Type:         "sqlite",
		MaxBytesSize: _maxBytesSize,
	}
}
