package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	c "gomarks/pkg/constants"
	"gomarks/pkg/utils"

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

var InitBookmark Bookmark = Bookmark{
	ID:    0,
	URL:   "https://github.com/haaag/GoMarks#readme",
	Title: NullString{NullString: sql.NullString{String: "GoMarks", Valid: true}},
	Tags:  "golang,awesome,bookmarks",
	Desc: NullString{
		sql.NullString{
			String: "Makes accessing, adding, updating, and removing bookmarks easier",
			Valid:  true,
		},
	},
}

type SQLiteRepository struct {
	DB *sql.DB
}

// https://medium.com/@raymondhartoyo/one-simple-way-to-handle-null-database-value-in-golang-86437ec75089
type Bookmark struct {
	ID         int        `json:"ID"`
	URL        string     `json:"URL"`
	Title      NullString `json:"Title"`
	Tags       string     `json:"Tags"`
	Desc       NullString `json:"Desc"`
	Created_at string     `json:"Created_at"`
}

func (b *Bookmark) CopyToClipboard() {
	err := clipboard.WriteAll(b.URL)
	if err != nil {
		log.Fatalf("Error copying to clipboard: %v", err)
	}
	log.Println("Text copied to clipboard:", b.URL)
}

func (b Bookmark) String() string {
	s := utils.PrettyFormatLine("ID", strconv.Itoa(b.ID))
	s += utils.PrettyFormatLine("Title", b.Title.String)
	s += utils.PrettyFormatLine("URL", b.URL)
	s += utils.PrettyFormatLine("Tags", b.Tags)
	s += utils.PrettyFormatLine("Desc", b.Desc.String)
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
		DB: db,
	}
}

func GetDB() *SQLiteRepository {
	dbPath, err := utils.GetDBPath()
	if err != nil {
		log.Fatal("Error getting database path:", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := NewSQLiteRepository(db)
	if !r.tableExists(c.DBMainTable) {
		r.initDB()
	}
	return r
}

func (r *SQLiteRepository) initDB() {
	_, err := r.DB.Exec(c.BookmarksSquema)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: Database initialized. Table: %s\n", c.AppName, c.DBMainTable)

	_, err = r.DB.Exec(c.DeletedBookmarksSchema)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s: Database initialized. Table: %s\n", c.AppName, c.DBDeletedTable)

	if _, err := r.InsertRecord(&InitBookmark, c.DBMainTable); err != nil {
		return
	}
}

func (r *SQLiteRepository) HandleDropDB() {
	_, err := r.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", c.DBMainTable))
	if err != nil {
		log.Fatal(err)
	}
	_, err = r.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", c.DBDeletedTable))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: Database dropped.\n", c.AppName)
}

func (r *SQLiteRepository) InsertRecord(b *Bookmark, tableName string) (Bookmark, error) {
	if r.isRecordExists(b, tableName) {
		return *b, fmt.Errorf(
			"bookmark already exists in %s table %s: %s",
			tableName,
			ErrDuplicate,
			b.URL,
		)
	}

	currentTime := time.Now()
	sqlQuery := fmt.Sprintf(
		`INSERT INTO %s(
      url, title, tags, desc, created_at)
      VALUES(?, ?, ?, ?, ?)`, tableName)
	result, err := r.DB.Exec(
		sqlQuery,
		b.URL,
		b.Title,
		b.Tags,
		b.Desc,
		currentTime.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return *b, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return *b, err
	}
  b.ID = int(id)

	log.Printf("Inserted bookmark: %s (table: %s)\n", b.URL, tableName)
	return *b, nil
}

func (r *SQLiteRepository) UpdateRecord(b *Bookmark) error {
	return nil
}

func (r *SQLiteRepository) DeleteRecord(b *Bookmark, tableName string) error {
	if !r.isRecordExists(b, tableName) {
		return fmt.Errorf("error removing bookmark %s: %s", ErrNotExists, b.URL)
	}
	sqlQuery := fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName)
	log.Printf("Deleted bookmark %s (table: %s)\n", b.URL, tableName)
	_, err := r.DB.Exec(sqlQuery, b.ID)
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) GetRecordByID(n int) (*Bookmark, error) {
	sqlQuery := fmt.Sprintf("SELECT * FROM %s WHERE id = ?", c.DBMainTable)
	row := r.DB.QueryRow(sqlQuery, n)
	var b Bookmark
	if err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *SQLiteRepository) getRecordsBySQL(q string, args ...interface{}) ([]Bookmark, error) {
	rows, err := r.DB.Query(q, args...)
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
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", c.DBMainTable)
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
		c.DBMainTable,
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
	err := r.DB.QueryRow(sqlQuery, b.URL).Scan(&recordCount)
	if err != nil {
		log.Fatal(err)
	}
	return recordCount > 0
}

func (r *SQLiteRepository) getMaxID() int {
	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", c.DBMainTable)
	var lastIndex int
	err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex)
	if err != nil {
		log.Fatal(err)
	}
	return lastIndex
}

func (r *SQLiteRepository) ReorderIDs() error {
	if r.getMaxID() == 0 {
		return nil
	}
	_, err := r.DB.Exec(c.TempBookmarksSchema)
	if err != nil {
		return err
	}
	bookmarks, err := r.getRecordsAll()
	if err != nil {
		return err
	}

	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}

	sqlQuery := fmt.Sprintf(
		"INSERT INTO temp_%s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		c.DBMainTable,
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

	_, err = r.DB.Exec(fmt.Sprintf("DROP TABLE %s", c.DBMainTable))
	if err != nil {
		return err
	}

	_, err = r.DB.Exec(fmt.Sprintf("ALTER TABLE temp_%s RENAME TO bookmarks", c.DBMainTable))
	if err != nil {
		return err
	}
	return nil
}

func (r *SQLiteRepository) tableExists(tableName string) bool {
	sqlQuery := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
	rows, err := r.DB.Query(sqlQuery, tableName)
	if err != nil {
		return false
	}
	defer rows.Close()
	return rows.Next()
}

func MigrateData(r *SQLiteRepository) {
	sqlQuery := fmt.Sprintf("SELECT url, title, tags, desc, created_at FROM %s", c.DBMainTable)

	sourceDB, err := sql.Open("sqlite3", "/home/void/.config/GoMarks/migrate.db")
	if err != nil {
		log.Fatal(err)
	}
	rows, err := sourceDB.Query(sqlQuery)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var b Bookmark
		if err := rows.Scan(&b.URL, &b.Title, &b.Tags, &b.Desc, &b.Created_at); err != nil {
			log.Fatal(err)
		}

		b, err := r.InsertRecord(&b, c.DBMainTable)
		if err != nil {
			log.Fatal(err)
		}
	}
}

func ToJSON(b *[]Bookmark) string {
	jsonData, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		log.Fatal("Error marshaling to JSON:", err)
	}
	jsonString := string(jsonData)
	return jsonString
}

func FetchBookmarks(r *SQLiteRepository, byQuery string) ([]Bookmark, error) {
	var bookmarks []Bookmark
	var err error

	if byQuery != "" {
		bookmarks, err = r.getRecordsByQuery(byQuery)
		if err != nil {
			return nil, err
		}
	} else {
		bookmarks, err = r.getRecordsAll()
		if err != nil {
			return nil, err
		}
	}

	if len(bookmarks) == 0 {
		return []Bookmark{}, fmt.Errorf("no bookmarks found")
	}
	return bookmarks, nil
}
