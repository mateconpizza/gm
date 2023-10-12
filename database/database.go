package database

import (
	"database/sql"
	"errors"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteRepository struct {
	db *sql.DB
}

type Bookmark struct {
	ID         int
	URL        string
	Title      sql.NullString
	Tags       sql.NullString
	Desc       sql.NullString
	Created_at sql.NullString
	Last_used  sql.NullString
}

func (b Bookmark) String() string {
	return fmt.Sprintf("ID: %d, URL: %s, Title: %s, Tags: %s, Desc: %s, Created_at: %s, Last_used: %s",
		b.ID, b.URL, validString(b.Title), validString(b.Tags), validString(b.Desc), validString(b.Created_at), validString(b.Last_used))
}

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

func validString(title sql.NullString) string {
	if title.Valid {
		return title.String
	}
	return "N/A"
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

func (r *SQLiteRepository) GetRecordsAll() ([]Bookmark, error) {
	rows, err := r.db.Query("SELECT id, url, title, tags, desc, created_at, last_used FROM bookmarks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Bookmark
	for rows.Next() {
		var bookmark Bookmark
		if err := rows.Scan(&bookmark.ID, &bookmark.URL, &bookmark.Title, &bookmark.Tags, &bookmark.Desc, &bookmark.Created_at, &bookmark.Last_used); err != nil {
			return nil, err
		}
		all = append(all, bookmark)
	}
	return all, nil
}

func (r *SQLiteRepository) GetRecordByID(id int) (*Bookmark, error) {
	row := r.db.QueryRow("SELECT id, url, title, tags, desc, created_at, last_used FROM bookmarks WHERE id = ?", id)
	var bookmark Bookmark
	if err := row.Scan(&bookmark.ID, &bookmark.URL, &bookmark.Title, &bookmark.Tags, &bookmark.Desc, &bookmark.Created_at, &bookmark.Last_used); err != nil {
		return nil, err
	}
	return &bookmark, nil
}
