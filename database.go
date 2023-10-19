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
	ErrDuplicate    = errors.New("sql: record already exists")
	ErrNotExists    = errors.New("sql: row not exists")
	ErrUpdateFailed = errors.New("sql: update failed")
	ErrDeleteFailed = errors.New("sql: delete failed")
)

type SQLiteRepository struct {
	db *sql.DB
}

// https://medium.com/@raymondhartoyo/one-simple-way-to-handle-null-database-value-in-golang-86437ec75089
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

	r := NewSQLiteRepository(db)
	if !r.tableExists(DBMainTable) {
		r.initDB()
	}
	return r
}

func (r *SQLiteRepository) initDB() {
	_, err := r.db.Exec(BookmarksSquema)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: Database initialized. Table: %s\n", AppName, DBMainTable)

	_, err = r.db.Exec(DeletedBookmarksSchema)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: Database initialized. Table: %s\n", AppName, DBDeletedTable)

	if err := r.insertRecord(&InitBookmark, DBMainTable); err != nil {
		return
	}
}

func (r *SQLiteRepository) dropDB() {
	_, err := r.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", DBMainTable))
	if err != nil {
		log.Fatal(err)
	}
	_, err = r.db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", DBDeletedTable))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: Database dropped.\n", AppName)
}

func (r *SQLiteRepository) insertRecord(b *Bookmark, tableName string) error {
	if r.isRecordExists(b, DBDeletedTable) {
		return fmt.Errorf("error inserting bookmark %s: %s", ErrDuplicate, b.URL)
	}

	currentTime := time.Now()
	sqlQuery := fmt.Sprintf(
		`INSERT INTO %s(
      url, title, tags, desc, created_at)
      VALUES(?, ?, ?, ?, ?)`, tableName)
	_, err := r.db.Exec(
		sqlQuery,
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		currentTime.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return err
	}
	fmt.Printf("Inserted bookmark: %s (table: %s)\n", b.URL, tableName)
	return nil
}

func (r *SQLiteRepository) updateRecord(b *Bookmark) error {
	return nil
}

func (r *SQLiteRepository) deleteRecord(b *Bookmark, tableName string) error {
	if !r.isRecordExists(b, tableName) {
		return fmt.Errorf("error removing bookmark %s: %s", ErrNotExists, b.URL)
	}
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)
	log.Printf("Deleted bookmark %s (table: %s)\n", b.URL, tableName)
	_, err := r.db.Exec(sqlQuery, b.ID)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) getRecordByID(n int) (*Bookmark, error) {
	sqlQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", DBMainTable)
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

func (r *SQLiteRepository) getRecordsAll() ([]Bookmark, error) {
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", DBMainTable)
	bookmarks, err := r.getRecordsBySQL(sqlQuery)
	if err != nil {
		log.Fatal(err)
	}
	if len(bookmarks) == 0 {
		return []Bookmark{}, nil
	}
	return bookmarks, nil
}

func (r *SQLiteRepository) getRecordsByQuery(q string) ([]Bookmark, error) {
	sqlQuery := fmt.Sprintf(
		`SELECT 
        id, url, title, tags, desc, created_at
      FROM %s 
      WHERE id LIKE ? 
        OR title LIKE ? 
        OR url LIKE ? 
        OR tags LIKE ? 
        OR desc LIKE ?
      ORDER BY id ASC
    `,
		DBMainTable,
	)
	queryValue := "%" + q + "%"
	return r.getRecordsBySQL(
		sqlQuery,
		queryValue,
		queryValue,
		queryValue,
		queryValue,
		queryValue,
	)
}

func (r *SQLiteRepository) isRecordExists(b *Bookmark, tableName string) bool {
	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE url=?", tableName)
	var recordCount int
	err := r.db.QueryRow(sqlQuery, b.URL).Scan(&recordCount)
	if err != nil {
		log.Fatal(err)
	}
	return recordCount > 0
}

func (r *SQLiteRepository) getMaxID() int {
	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", DBMainTable)
	var lastIndex int
	err := r.db.QueryRow(sqlQuery).Scan(&lastIndex)
	if err != nil {
		log.Fatal(err)
	}
	return lastIndex
}

func (r *SQLiteRepository) reorderIDs() error {
	if r.getMaxID() == 0 {
		return nil
	}
	_, err := r.db.Exec(TempBookmarksSchema)
	if err != nil {
		return err
	}
	bookmarks, err := r.getRecordsAll()
	if err != nil {
		return err
	}

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}

	sqlQuery := fmt.Sprintf(
		"INSERT INTO temp_%s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		DBMainTable,
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

	_, err = r.db.Exec(fmt.Sprintf("DROP TABLE %s", DBMainTable))
	if err != nil {
		return err
	}

	_, err = r.db.Exec(fmt.Sprintf("ALTER TABLE temp_%s RENAME TO bookmarks", DBMainTable))
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) tableExists(tableName string) bool {
	sqlQuery := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	rows, err := r.db.Query(sqlQuery, tableName)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}
