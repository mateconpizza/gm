package repo

import (
	"context"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/internal/format/color"
)

// SQLiteRepository implements the Repository interface.
type SQLiteRepository struct {
	DB     *sqlx.DB      `json:"-"`
	Cfg    *SQLiteConfig `json:"db"`
	closed bool
}

// String returns a string representation of the repository.
func (r *SQLiteRepository) String() string {
	t := r.Cfg.Tables
	main := fmt.Sprintf("(main: %d, ", CountRecords(r, t.Main))
	deleted := fmt.Sprintf("deleted: %d)", CountRecords(r, t.Deleted))
	records := color.Gray(main + deleted).Italic()

	return r.Cfg.Name + " " + records.String()
}

// IsClosed checks if the database connection is closed.
func (r *SQLiteRepository) IsClosed() bool {
	return r.closed
}

// Close closes the SQLite database connection and logs any errors encountered.
func (r *SQLiteRepository) Close() {
	s := r.Cfg.Name
	if r.IsClosed() {
		log.Printf("database '%s' already closed.\n", s)
		return
	}
	if err := r.DB.Close(); err != nil {
		log.Printf("closing '%s' database: %v", s, err)
	}

	r.closed = true
	log.Printf("database '%s' closed.\n", s)
}

func (r *SQLiteRepository) SetMain(t Table) {
	log.Printf("main table set to: %s", t)
	r.Cfg.Tables.Main = t
}

func (r *SQLiteRepository) SetDeleted(t Table) {
	log.Printf("deleted table set to: %s", t)
	r.Cfg.Tables.Deleted = t
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sqlx.DB, cfg *SQLiteConfig) *SQLiteRepository {
	return &SQLiteRepository{
		DB:     db,
		Cfg:    cfg,
		closed: false,
	}
}

// New creates a new `SQLiteRepository` using the provided configuration and
// opens the database, returning the repository or an error.
func New(c *SQLiteConfig) (*SQLiteRepository, error) {
	db, err := MustOpenDatabase(c.Fullpath())
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.maintenance(); err != nil {
		return nil, err
	}

	return r, nil
}

// MustOpenDatabase opens a SQLite database at the specified path and verifies
// the connection, returning the database handle or an error.
func MustOpenDatabase(s string) (*sqlx.DB, error) {
	log.Printf("opening database: '%s'", s)
	db, err := sqlx.Open("sqlite3", s)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		panic(fmt.Errorf("%w: on ping context", err))
	}

	return db, nil
}
