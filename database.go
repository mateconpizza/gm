package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
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
	log.Println("Text copied to clipboard:", b.URL)
}

func (b Bookmark) String() string {
	s := prettyFormatLine("ID", strconv.Itoa(b.ID))
	s += prettyFormatLine("Title", b.Title.String)
	s += prettyFormatLine("URL", b.URL)
	s += prettyFormatLine("Tags", b.Tags)
	s += prettyFormatLine("Desc", b.Desc.String)
	return s
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

func getDB() *SQLiteRepository {
	dbPath, err := getDBPath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	bookmarksRepo := NewSQLiteRepository(db)
	bookmarksRepo.InitDB()
	return bookmarksRepo
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
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id = ?", DBTableName)
	_, err := r.db.Exec(sqlQuery, b.ID)
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
	sqlQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", DBTableName)
	row := r.db.QueryRow(sqlQuery, n)
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
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", DBTableName)
	bookmarks, err := r.getRecordsBySQL(sqlQuery)
	if err != nil {
		log.Fatal(err)
	}
	if len(bookmarks) == 0 {
		return []Bookmark{}, nil
	}
	return bookmarks, nil
}

func (r *SQLiteRepository) GetRecordsByQuery(q string) ([]Bookmark, error) {
	sqlQuery := fmt.Sprintf(
		"SELECT id, url, title, tags, desc, created_at FROM %s WHERE title LIKE ? OR url LIKE ? or tags LIKE ? or desc LIKE ?",
		DBTableName,
	)
	queryValue := "%" + q + "%"
	return r.getRecordsBySQL(
		sqlQuery,
		queryValue,
		queryValue,
		queryValue,
		queryValue,
	)
}

func (r *SQLiteRepository) RecordExists(b *Bookmark) bool {
	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE url=?", DBTableName)
	var recordCount int
	err := r.db.QueryRow(sqlQuery, b.URL).Scan(&recordCount)
	if err != nil {
		log.Fatal(err)
	}
	return recordCount > 0
}

func (r *SQLiteRepository) GetMaxID() int {
	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", DBTableName)
	var lastIndex int
	err := r.db.QueryRow(sqlQuery).Scan(&lastIndex)
	if err != nil {
		log.Fatal(err)
	}
	return lastIndex
}

func (r *SQLiteRepository) RemoveAllRecords() error {
	sqlQuery := fmt.Sprintf("DELETE FROM %s", DBTableName)
	_, err := r.db.Exec(sqlQuery)
	if err != nil {
		log.Fatal(err)
	}
	_, err = r.db.Exec(fmt.Sprintf("DELETE FROM SQLITE_SEQUENCE WHERE NAME = '%s'", DBTableName))
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func (r *SQLiteRepository) ReorderIDs() error {
	if r.GetMaxID() == 0 {
		return nil
	}

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

	sqlQuery := fmt.Sprintf(
		"INSERT INTO temp_%s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		DBTableName,
	)
	stmt, err := tx.Prepare(sqlQuery)
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

	_, err = r.db.Exec(fmt.Sprintf("DROP TABLE %s", DBTableName))
	if err != nil {
		return err
	}

	_, err = r.db.Exec(fmt.Sprintf("ALTER TABLE temp_%s RENAME TO bookmarks", DBTableName))
	if err != nil {
		return err
	}

	return nil
}
