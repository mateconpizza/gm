package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/internal/sys/files"
)

var connClosed bool

// SQLiteRepository implements the Repository interface.
type SQLiteRepository struct {
	DB  *sql.DB       `json:"-"`
	Cfg *SQLiteConfig `json:"db"`
}

// Close closes the SQLite database connection and logs any errors encountered.
func (r *SQLiteRepository) Close() {
	if err := r.DB.Close(); err != nil {
		log.Printf("closing database: %v", err)
	}

	connClosed = true
	log.Printf("database closed.")
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sql.DB, cfg *SQLiteConfig) *SQLiteRepository {
	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}
}

// New creates a new `SQLiteRepository` using the provided configuration and
// opens the database, returning the repository or an error.
func New(c *SQLiteConfig) (*SQLiteRepository, error) {
	c.Name = files.AddExtension(c.Name, ".db")
	db, err := MustOpenDatabase(c.Fullpath())
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.maintenance(c); err != nil {
		return nil, err
	}

	return r, nil
}

// MustOpenDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func MustOpenDatabase(s string) (*sql.DB, error) {
	log.Printf("opening database: '%s'", s)
	db, err := sql.Open("sqlite3", s)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		panic(fmt.Errorf("%w: on ping context", err))
	}

	return db, nil
}
