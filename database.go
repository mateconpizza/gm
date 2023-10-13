package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// [TODO):
// [ ] Add Tables (bookmarks, deleted)
// [ ] Add CRUD methods
// [ ] Add tests
// [X] Add error handling

type SQLiteRepository struct {
	db *sql.DB
}

type Bookmark struct {
	ID         int
	URL        string
	Title      string
	Tags       string
	Desc       string
	Created_at sql.NullString
	Last_used  sql.NullString
}

var (
	ErrDuplicate    = errors.New("::::record already exists")
	ErrNotExists    = errors.New("::::row not exists")
	ErrUpdateFailed = errors.New("::::update failed")
	ErrDeleteFailed = errors.New("::::delete failed")
)

func (b Bookmark) String() string {
	return fmt.Sprintf(
		"ID: %d, URL: %s, Title: %s, Tags: %s, Desc: %s, Created_at: %s, Last_used: %s",
		b.ID,
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		validString(b.Created_at),
		validString(b.Last_used),
	)
}

func validString(title sql.NullString) string {
	if title.Valid {
		return title.String
	}
	return ""
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		db: db,
	}
}

func (r *SQLiteRepository) InitDB() {
	_, err := r.db.Exec(BookmarksSquema)
	if err != nil {
		log.Fatal(err)
	}
	if err := r.CreateRecord(&InitBookmark); err != nil {
		return
	}
}

func (r *SQLiteRepository) CreateRecord(b *Bookmark) error {
	if r.RecordExists(b) && b.URL != InitBookmark.URL {
		log.Println(ErrDuplicate, b.URL)
		return nil
	}

	currentTime := time.Now()
	_, err := r.db.Exec(
		"INSERT INTO bookmarks(url, title, tags, desc, created_at, last_used) VALUES(?, ?, ?, ?, ?, ?)",
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		currentTime.Format("2006-01-02 15:04:05"),
		b.Last_used,
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) RemoveRecord(b *Bookmark) error {
	if !r.RecordExists(b) {
		log.Println(ErrNotExists, b.URL)
		return nil
	}
	_, err := r.db.Exec("DELETE FROM bookmarks WHERE id = ?", b.ID)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) GetRecordByID(n int) (*Bookmark, error) {
	row := r.db.QueryRow(
		"SELECT id, url, title, tags, desc, created_at, last_used FROM bookmarks WHERE id = ?",
		n,
	)
	var b Bookmark
	if err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at, &b.Last_used); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *SQLiteRepository) getRecordsBySQL(q string, args ...interface{}) ([]Bookmark, error) {
	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []Bookmark
	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at, &b.Last_used); err != nil {
			return nil, err
		}
		all = append(all, b)
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("no bookmarks found for query: %s", args[0])
	}
	return all, nil
}

func (r *SQLiteRepository) GetRecordsAll() ([]Bookmark, error) {
	return r.getRecordsBySQL(
		"SELECT id, url, title, tags, desc, created_at, last_used FROM bookmarks",
	)
}

func (r *SQLiteRepository) GetRecordsByQuery(q string) ([]Bookmark, error) {
	queryValue := "%" + q + "%"
	return r.getRecordsBySQL(
		"SELECT id, url, title, tags, desc, created_at, last_used FROM bookmarks WHERE title LIKE ? OR url LIKE ? or tags LIKE ? or desc LIKE ?",
		queryValue,
		queryValue,
		queryValue,
		queryValue,
	)
}

func (r *SQLiteRepository) RecordExists(b *Bookmark) bool {
	var query string = "SELECT COUNT(*) FROM bookmarks WHERE url=?"
	var recordCount int
	err := r.db.QueryRow(query, b.URL).Scan(&recordCount)
	if err != nil {
		log.Fatal(err)
	}
	return recordCount > 0
}
