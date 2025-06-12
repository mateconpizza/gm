package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"

	"github.com/mateconpizza/gm/internal/sys/files"
)

//nolint:errcheck //test
func createTestSQLiteDB(t *testing.T, dir, dbName string) string {
	t.Helper()
	dbPath := filepath.Join(dir, dbName)

	// Open a connection
	db, err := sqlx.Open("sqlite3", dbPath)
	assert.NoError(t, err, "Failed to open SQLite DB for test setup at %s", dbPath)
	defer db.Close() // Ensure the DB connection is closed when this helper returns

	// Ping to ensure the connection is valid and the file is accessible as a DB
	err = db.PingContext(t.Context())
	assert.NoError(t, err, "Failed to ping SQLite DB for test setup at %s", dbPath)

	// Create a dummy table to make it a valid SQLite database file
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT);`)
	assert.NoError(t, err, "Failed to create dummy table in test DB at %s", dbPath)

	return dbPath
}

func TestNewRepository(t *testing.T) {
	t.Run("empty path returns ErrPathEmpty", func(t *testing.T) {
		t.Parallel()
		r, err := New("")
		assert.Nil(t, r)
		assert.Error(t, err)
		assert.ErrorIs(t, err, files.ErrPathEmpty)
	})

	t.Run("non-existent path returns ErrPathNotFound", func(t *testing.T) {
		t.Parallel()
		r, err := New("/tmp/invalid/path")
		assert.Nil(t, r)
		assert.Error(t, err)
		assert.ErrorIs(t, err, ErrDBNotFound)
	})

	t.Run("valid path to non-sqlite file returns error", func(t *testing.T) {
		t.Parallel()
		nonSqlitePath := filepath.Join(t.TempDir(), "not_a_db.txt")
		err := os.WriteFile(nonSqlitePath, []byte("This is not a database."), files.FilePerm)
		if !assert.NoError(t, err, "Failed to create non-sqlite file for test") {
			assert.FailNow(t, "Failed to create non-sqlite file for test", "error", err)
		}
		assert.True(t, files.Exists(nonSqlitePath))
		r, err := New(nonSqlitePath)
		assert.Nil(t, r)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "file is not a database")
	})

	t.Run("valid path to empty sqlite file returns repository", func(t *testing.T) {
		t.Parallel()
		d := t.TempDir()
		s := "test_db.sqlite"
		dbPath := createTestSQLiteDB(t, d, s)
		r, err := New(dbPath)
		assert.NoError(t, err)
		assert.NotNil(t, r)
		assert.NotNil(t, r.DB)
		assert.NotNil(t, r.Cfg)
		assert.Equal(t, r.Cfg.Fullpath(), filepath.Join(d, s))

		// Optionally, ping the DB connection within the returned repository
		// to ensure it's still alive and functional.
		err = r.DB.PingContext(t.Context())
		assert.NoError(t, err, "Repository DB connection should be pingable after creation")
	})
}
