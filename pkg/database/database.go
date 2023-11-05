package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/constants"
	"gomarks/pkg/util"

	_ "github.com/mattn/go-sqlite3"
)

var (
	ErrDuplicate    = errors.New("sql: record already exists")
	ErrNotExists    = errors.New("sql: row not exists")
	ErrUpdateFailed = errors.New("sql: update failed")
	ErrDeleteFailed = errors.New("sql: delete failed")
)

type SQLiteRepository struct {
	DB *sql.DB
}

func newSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{
		DB: db,
	}
}

func GetDB() *SQLiteRepository {
	dbPath := util.GetDBPath()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	r := newSQLiteRepository(db)
	if exists, _ := r.tableExists(constants.DBMainTableName); !exists {
		r.initDB()
		fmt.Printf("Initialized database: %s", dbPath)
	}

	return r
}

func (r *SQLiteRepository) initDB() {
	if err := r.createTable(constants.DBMainTableName); err != nil {
		log.Fatal(err)
	}

	if err := r.createTable(constants.DBDeletedTableName); err != nil {
		log.Fatal(err)
	}

	if _, err := r.InsertRecord(&bookmark.InitBookmark, constants.DBMainTableName); err != nil {
		return
	}
}

func (r *SQLiteRepository) dropTable(t string) error {
	log.Printf("Dropping table: %s", t)

	_, err := r.DB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", t))
	if err != nil {
		return err
	}

	log.Printf("Dropped table: %s\n", t)

	return nil
}

func (r *SQLiteRepository) InsertRecord(
	b *bookmark.Bookmark,
	tableName string,
) (*bookmark.Bookmark, error) {
	if !b.IsValid() {
		return nil, fmt.Errorf("invalid bookmark: %w", ErrNotExists)
	}

	if r.RecordExists(b.URL, tableName) {
		return nil, fmt.Errorf(
			"bookmark already exists in %s table %w: %s",
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
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	b.ID = int(id)

	log.Printf("Inserted bookmark: %s (table: %s)\n", b.URL, tableName)

	return b, nil
}

func (r *SQLiteRepository) UpdateRecord(
	b *bookmark.Bookmark,
	t string,
) (*bookmark.Bookmark, error) {
	if !r.RecordExists(b.URL, t) {
		return b, fmt.Errorf("error updating bookmark %w: %s", ErrNotExists, b.URL)
	}

	sqlQuery := fmt.Sprintf(
		"UPDATE %s SET url = ?, title = ?, tags = ?, desc = ?, created_at = ? WHERE id = ?",
		t,
	)

	_, err := r.DB.Exec(sqlQuery, b.URL, b.Title.String, b.Tags, b.Desc.String, b.CreatedAt, b.ID)
	if err != nil {
		return b, fmt.Errorf("error updating bookmark %w: %w", ErrUpdateFailed, err)
	}

	log.Printf("Updated bookmark %s (table: %s)\n", b.URL, t)

	return b, nil
}

func (r *SQLiteRepository) DeleteRecord(b *bookmark.Bookmark, tableName string) error {
	log.Printf("Deleting bookmark %s (table: %s)\n", b.URL, tableName)

	if !r.RecordExists(b.URL, tableName) {
		return fmt.Errorf("error removing bookmark %w: %s", ErrNotExists, b.URL)
	}

	_, err := r.DB.Exec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", tableName), b.ID)
	if err != nil {
		return fmt.Errorf("error removing bookmark %w: %w", ErrDeleteFailed, err)
	}

	log.Printf("Deleted bookmark %s (table: %s)\n", b.URL, tableName)

	return nil
}

func (r *SQLiteRepository) GetRecordByID(n int, t string) (*bookmark.Bookmark, error) {
	log.Printf("Getting bookmark by ID %d (table: %s)\n", n, t)
	row := r.DB.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE id = ?", t), n)

	var b bookmark.Bookmark

	err := row.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("bookmark with ID %d not found", n)
		}

		return nil, err
	}

	log.Printf("Got bookmark by ID %d (table: %s)\n", n, t)

	return &b, nil
}

func (r *SQLiteRepository) getRecordsBySQL(
	q string,
	args ...interface{},
) (*bookmark.Slice, error) {
	rows, err := r.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	var all bookmark.Slice

	for rows.Next() {
		var b bookmark.Bookmark
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Tags, &b.Desc, &b.CreatedAt); err != nil {
			return nil, err
		}

		all = append(all, b)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &all, nil
}

func (r *SQLiteRepository) GetRecordsAll(t string) (*bookmark.Slice, error) {
	log.Printf("Getting all records from table: '%s'", t)
	sqlQuery := fmt.Sprintf("SELECT * FROM %s ORDER BY id ASC", t)

	bs, err := r.getRecordsBySQL(sqlQuery)
	if err != nil {
		return nil, err
	}

	if bs.Len() == 0 {
		log.Printf("No records found in table: '%s'", t)

		return nil, fmt.Errorf("no bookmarks found")
	}

	log.Printf("Got %d records from table: '%s'", bs.Len(), t)

	return bs, nil
}

func (r *SQLiteRepository) GetRecordsByQuery(q, t string) (*bookmark.Slice, error) {
	log.Printf("Getting records by query: %s", q)

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
		t,
	)
	queryValue := "%" + q + "%"
	bs, err := r.getRecordsBySQL(
		sqlQuery,
		queryValue,
		queryValue,
		queryValue,
		queryValue,
		queryValue,
	)
	if err != nil {
		return nil, err
	}

	if bs.Len() == 0 {
		log.Printf("No records found by query: '%s'", q)
		return nil, fmt.Errorf("no bookmarks found")
	}

	log.Printf("Got %d records by query: '%s'", bs.Len(), q)

	return bs, err
}

func (r *SQLiteRepository) RecordExists(url, tableName string) bool {
	var recordCount int

	sqlQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE url=?", tableName)

	err := r.DB.QueryRow(sqlQuery, url).Scan(&recordCount)
	if err != nil {
		log.Fatal(err)
	}

	return recordCount > 0
}

func (r *SQLiteRepository) getMaxID() int {
	var lastIndex int

	sqlQuery := fmt.Sprintf("SELECT COALESCE(MAX(id), 0) FROM %s", constants.DBMainTableName)

	err := r.DB.QueryRow(sqlQuery).Scan(&lastIndex)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Max ID: %d", lastIndex)

	return lastIndex
}

func (r *SQLiteRepository) tableExists(t string) (bool, error) {
	log.Printf("Checking if table '%s' exists", t)

	query := "SELECT name FROM sqlite_master WHERE type='table' AND name = ?"

	rows, err := r.DB.Query(query, t)
	if err != nil {
		return false, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	if !rows.Next() {
		return false, nil
	}

	if err := rows.Err(); err != nil {
		return false, err
	}

	log.Printf("Table '%s' exists", t)

	return true, nil
}

func (r *SQLiteRepository) insertRecordsBulk(
	tempTable string,
	bs *bookmark.Slice,
) error {
	log.Printf("Inserting %d bookmarks into table: %s", bs.Len(), tempTable)

	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}

	sqlQuery := fmt.Sprintf(
		"INSERT INTO %s (url, title, tags, desc, created_at) VALUES (?, ?, ?, ?, ?)",
		tempTable,
	)

	stmt, err := tx.Prepare(sqlQuery)
	if err != nil {
		err = tx.Rollback()
		return err
	}

	for _, b := range *bs {
		_, err = stmt.Exec(b.URL, b.Title, b.Tags, b.Desc, b.CreatedAt)
		if err != nil {
			err = tx.Rollback()
			return err
		}
	}

	defer func() {
		if err := stmt.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	if err := stmt.Close(); err != nil {
		err = tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("Inserted %d bookmarks into table: %s", bs.Len(), tempTable)

	return nil
}

func (r *SQLiteRepository) renameTable(tempTable, mainTable string) error {
	log.Printf("Renaming table %s to %s", tempTable, mainTable)

	_, err := r.DB.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s", tempTable, mainTable))
	if err != nil {
		return err
	}

	log.Printf("Renamed table %s to %s\n", tempTable, mainTable)

	return nil
}

func (r *SQLiteRepository) createTable(s string) error {
	log.Printf("Creating table: %s", s)
	schema := fmt.Sprintf(constants.MainTableSchema, s)

	_, err := r.DB.Exec(schema)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	log.Printf("Created table: %s\n", s)

	return err
}

func (r *SQLiteRepository) ReorderIDs() error {
	log.Printf("Reordering IDs in table: %s", constants.DBMainTableName)

	if r.getMaxID() == 0 {
		return nil
	}

	tempTable := fmt.Sprintf("temp_%s", constants.DBMainTableName)
	if err := r.createTable(tempTable); err != nil {
		return err
	}

	bookmarks, err := r.GetRecordsAll(constants.DBMainTableName)
	if err != nil {
		return err
	}

	if err := r.insertRecordsBulk(tempTable, bookmarks); err != nil {
		return err
	}

	if err := r.dropTable(constants.DBMainTableName); err != nil {
		return err
	}

	return r.renameTable(tempTable, constants.DBMainTableName)
}

func (r *SQLiteRepository) TagsWithCount() (util.Counter, error) {
	// FIX: make it local
	tagCounter := make(util.Counter)

	bs, err := r.GetRecordsAll(constants.DBMainTableName)
	if err != nil {
		return nil, err
	}

	for _, bookmark := range *bs {
		tagCounter.Add(bookmark.Tags, ",")
	}

	return tagCounter, nil
}

func (r *SQLiteRepository) GetRecordsByTag(t string) (*bookmark.Slice, error) {
	// FIX: make it local
	bs, err := r.getRecordsBySQL(
		fmt.Sprintf("SELECT * FROM %s WHERE tags LIKE ?", constants.DBMainTableName),
		fmt.Sprintf("%%%s%%", t),
	)
	if err != nil {
		return nil, err
	}

	if bs.Len() == 0 {
		return nil, fmt.Errorf("no bookmarks found")
	}

	return bs, nil
}

func (r *SQLiteRepository) GetRecordsLength(t string) (int, error) {
	var length int

	row := r.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t))

	err := row.Scan(&length)
	if err != nil {
		return 0, err
	}

	return length, nil
}

func (r *SQLiteRepository) GetRecordsWithoutTitleOrDesc(t string) (*bookmark.Slice, error) {
	query := fmt.Sprintf("SELECT * from %s WHERE title IS NULL or desc IS NULL", t)

	bs, err := r.getRecordsBySQL(query)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

func (r *SQLiteRepository) GetUniqueTags(t string) ([]string, error) {
	var s []string

	query := fmt.Sprintf("SELECT tags from %s", t)
	rows, err := r.DB.Query(query, t)
	if err != nil {
		return []string{}, err
	}

	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Error closing rows: %v", err)
		}
	}()

	for rows.Next() {
		var tags string

		err := rows.Scan(&tags)
		if err != nil {
			return []string{}, err
		}

		s = append(s, tags)
	}

	if err := rows.Err(); err != nil {
		return []string{}, err
	}

	return util.ParseUniqueStrings(s, ","), nil
}
