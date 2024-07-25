package repo

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"

	"github.com/haaag/gm/pkg/util"
)

// SQLiteRepository implements the Repository interface.
type SQLiteRepository struct {
	DB  *sql.DB       `json:"-"`
	Cfg *SQLiteConfig `json:"db"`
}

func (r *SQLiteRepository) Close() {
	if err := r.DB.Close(); err != nil {
		log.Printf("closing database: %v", err)
	}
}

// newSQLiteRepository returns a new SQLiteRepository.
func newSQLiteRepository(db *sql.DB, cfg *SQLiteConfig) *SQLiteRepository {
	return &SQLiteRepository{
		DB:  db,
		Cfg: cfg,
	}
}

// New returns a new SQLiteRepository.
func New(c *SQLiteConfig) (*SQLiteRepository, error) {
	c.Name = util.EnsureDBSuffix(c.Name)
	db, err := MustOpenDatabase(filepath.Join(c.Path, c.Name))
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := newSQLiteRepository(db, c)
	if err := r.maintenance(c); err != nil {
		return nil, err
	}

	return r, nil
}

// MustOpenDatabase opens a database.
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
