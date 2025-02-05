package repo

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// FIX: rethink/redo SQLiteRepository|SQLiteConfig

// SQLiteRepository implements the Repository interface.
type SQLiteRepository struct {
	DB        *sqlx.DB      `json:"-"`
	Cfg       *SQLiteConfig `json:"db"`
	closeOnce sync.Once
}

// Close closes the SQLite database connection and logs any errors encountered.
func (r *SQLiteRepository) Close() {
	s := r.Cfg.Name
	r.closeOnce.Do(func() {
		if err := r.DB.Close(); err != nil {
			log.Printf("closing '%s' database: %v", s, err)
		} else {
			log.Printf("database '%s' closed.\n", s)
		}
	})
}

// SetMain sets the main table.
func (r *SQLiteRepository) SetMain(t Table) {
	log.Printf("main table set to: %s", t)
	r.Cfg.Tables.Main = t
}

// SetDeleted sets the deleted table.
func (r *SQLiteRepository) SetDeleted(t Table) {
	log.Printf("deleted table set to: %s", t)
	r.Cfg.Tables.Deleted = t
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sqlx.DB, cfg *SQLiteConfig) *SQLiteRepository {
	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}
}

// New creates a new `SQLiteRepository` using the provided configuration and
// opens the database, returning the repository or an error.
func New(c *SQLiteConfig) (*SQLiteRepository, error) {
	db, err := openDatabase(c.Fullpath())
	if err != nil {
		log.Println("error opening database:", err)
		return nil, err
	}

	r := newSQLiteRepository(db, c)
	if err := r.maintenance(); err != nil {
		return nil, err
	}

	return r, nil
}

// openDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func openDatabase(s string) (*sqlx.DB, error) {
	log.Printf("opening database: '%s'", s)
	db, err := sqlx.Open("sqlite3", s)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("%w: on ping context", err)
	}

	return db, nil
}
