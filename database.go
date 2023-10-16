package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/atotto/clipboard"
	_ "github.com/mattn/go-sqlite3"
)

// [TODO):
// [X] Add Tables (bookmarks, deleted)
// [X] Add CRUD methods
// [ ] Add tests
// [X] Add error handling

var (
	ErrDuplicate    = errors.New("record already exists")
	ErrNotExists    = errors.New("row not exists")
	ErrUpdateFailed = errors.New("update failed")
	ErrDeleteFailed = errors.New("delete failed")
)

type SQLiteRepository struct {
	db *sql.DB
}

type Bookmark struct {
	ID         int        `json:"ID,omitempty"`
	URL        string     `json:"URL,omitempty"`
	Title      NullString `json:"Title,omitempty"`
	Tags       string     `json:"Tags,omitempty"`
	Desc       NullString `json:"Desc,omitempty"`
	Created_at NullString `json:"Created_at,omitempty"`
}

func (b *Bookmark) CopyToClipboard() {
	err := clipboard.WriteAll(b.URL)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}
	log.Println("Text copied to clipboard:", b)
}

type NullString struct {
	sql.NullString
}

func (s NullString) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(s.String)
}

func (s *NullString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		s.String, s.Valid = "", false
		return nil
	}
	s.String, s.Valid = string(data), true
	return nil
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
	_, err = r.db.Exec(DeletedBookmarksSchema)
	if err != nil {
		log.Fatal(err)
	}
	/* if err := r.CreateRecord(&InitBookmark); err != nil {
		return
	} */
}

func (r *SQLiteRepository) CreateRecord(b *Bookmark) error {
	if r.RecordExists(b) && b.URL != InitBookmark.URL {
		log.Println(ErrDuplicate, b.URL)
		return nil
	}

	currentTime := time.Now()
	_, err := r.db.Exec(
		"INSERT INTO bookmarks(url, title, tags, desc, created_at) VALUES(?, ?, ?, ?, ?)",
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		currentTime.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) UpdateRecord(b *Bookmark) error {
	return nil
}

func (r *SQLiteRepository) RemoveRecord(b *Bookmark) error {
	if !r.RecordExists(b) {
		return fmt.Errorf("error removing bookmark %s: %s", ErrNotExists, b.URL)
	}
	_, err := r.db.Exec("DELETE FROM bookmarks WHERE id = ?", b.ID)
	if err != nil {
		return err
	}

	log.Println("Deleted bookmark", b.URL)

	err = r.ReorderIDs()
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) GetRecordByID(n int) (*Bookmark, error) {
	row := r.db.QueryRow(
		"SELECT id, url, title, tags, desc, created_at FROM bookmarks WHERE id = ?",
		n,
	)
	var b Bookmark
	if err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at); err != nil {
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
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at); err != nil {
			return nil, err
		}
		all = append(all, b)
	}
	return all, nil
}

func (r *SQLiteRepository) GetRecordsAll() ([]Bookmark, error) {
	bookmarks, err := r.getRecordsBySQL(
		"SELECT id, url, title, tags, desc, created_at FROM bookmarks",
	)
	if err != nil {
		log.Fatal(err)
	}
	if len(bookmarks) == 0 {
		var b []Bookmark
		b = append(b, InitBookmark)
		return b, nil
	}
	return bookmarks, nil
}

func (r *SQLiteRepository) GetRecordsByQuery(q string) ([]Bookmark, error) {
	queryValue := "%" + q + "%"
	return r.getRecordsBySQL(
		"SELECT id, url, title, tags, desc, created_at FROM bookmarks WHERE title LIKE ? OR url LIKE ? or tags LIKE ? or desc LIKE ?",
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

func (r *SQLiteRepository) GetMaxID() int {
	var lastIndex int
	err := r.db.QueryRow("SELECT MAX(id) FROM bookmarks").Scan(&lastIndex)
	if err != nil {
		log.Fatal(err)
	}
	return lastIndex
}

func (r *SQLiteRepository) RemoveAllRecords() error {
	_, err := r.db.Exec("DELETE FROM bookmarks")
	if err != nil {
		log.Fatal(err)
	}
	_, err = r.db.Exec("DELETE FROM SQLITE_SEQUENCE WHERE NAME = 'bookmarks'")
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (r *SQLiteRepository) ReorderIDs() error {
	_, err := r.db.Exec(TempBookmarksSchema)
	if err != nil {
		return err
	}

	bookmarks, err := r.GetRecordsAll()
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(
		"INSERT INTO temp_bookmarks (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
	)
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}
		return err
	}

	for _, b := range bookmarks {
		_, err = stmt.Exec(b.URL, b.Title, b.Tags, b.Desc, b.Created_at)
		if err != nil {
			err = tx.Rollback()
			if err != nil {
				return err
			}
			return err
		}
	}

	err = stmt.Close()
	if err != nil {
		err = tx.Rollback()
		if err != nil {
			return err
		}
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	_, err = r.db.Exec("DROP TABLE bookmarks")
	if err != nil {
		return err
	}

	_, err = r.db.Exec("ALTER TABLE temp_bookmarks RENAME TO bookmarks")
	if err != nil {
		return err
	}

	return nil
}
