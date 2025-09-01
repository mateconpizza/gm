//nolint:wsl,funlen //test
package db

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmoiron/sqlx"
)

//nolint:errcheck //test
func createTestSQLiteDB(t *testing.T, dir, dbName string) string {
	t.Helper()
	dbPath := filepath.Join(dir, dbName)

	// Open a connection
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open SQLite DB for test setup at %s: %v", dbPath, err)
	}
	defer db.Close()

	// Ping to ensure the connection is valid and the file is accessible as a DB
	err = db.PingContext(t.Context())
	if err != nil {
		t.Fatalf("Failed to ping SQLite DB for test setup at %s: %v", dbPath, err)
	}

	// Create a dummy table to make it a valid SQLite database file
	_, err = db.ExecContext(
		t.Context(),
		`CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT);`,
	)
	if err != nil {
		t.Fatalf("Failed to create dummy table in test DB at %s: %v", dbPath, err)
	}

	return dbPath
}

func TestNewRepository(t *testing.T) {
	t.Parallel()

	t.Run("empty path returns ErrPathEmpty", func(t *testing.T) {
		t.Parallel()
		r, err := New("")
		if r != nil {
			t.Errorf("expected nil repository, got: %v", r)
		}
		if err == nil || !errors.Is(err, ErrDBNotFound) {
			t.Errorf("expected error %v, got: %v", ErrDBNotFound, err)
		}
	})

	t.Run("non-existent path returns ErrDBNotFound", func(t *testing.T) {
		t.Parallel()
		r, err := New("/tmp/invalid/path")
		if r != nil {
			t.Errorf("expected nil repository, got: %v", r)
		}
		if err == nil || !errors.Is(err, ErrDBNotFound) {
			t.Errorf("expected error %v, got: %v", ErrDBNotFound, err)
		}
	})

	t.Run("valid path to non-sqlite file returns error", func(t *testing.T) {
		t.Parallel()
		nonSqlitePath := filepath.Join(t.TempDir(), "not_a_db.txt")
		err := os.WriteFile(nonSqlitePath, []byte("This is not a database."), 0o600)
		if err != nil {
			t.Fatalf("failed to create non-sqlite file for test: %v", err)
		}
		if !fileExists(nonSqlitePath) {
			t.Fatalf("expected file to exist: %s", nonSqlitePath)
		}
		r, err := New(nonSqlitePath)
		if r != nil {
			t.Errorf("expected nil repository, got: %v", r)
		}
		if err == nil || !strings.Contains(err.Error(), "file is not a database") {
			t.Errorf("expected sqlite error, got: %v", err)
		}
	})

	t.Run("valid path to empty sqlite file returns repository", func(t *testing.T) {
		t.Parallel()
		d := t.TempDir()
		s := "test_db.sqlite"
		dbPath := createTestSQLiteDB(t, d, s)
		r, err := New(dbPath)
		if err != nil {
			t.Fatalf("unexpected error creating repository: %v", err)
		}
		if r == nil || r.DB == nil || r.Cfg == nil {
			t.Fatal("repository or its fields are nil")
		}
		expectedPath := filepath.Join(d, s)
		if r.Cfg.Fullpath() != expectedPath {
			t.Errorf("expected db path %q, got %q", expectedPath, r.Cfg.Fullpath())
		}
		if err := r.DB.PingContext(t.Context()); err != nil {
			t.Errorf("failed to ping DB: %v", err)
		}
	})
}

func TestBuildSQLiteDSN(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		path   string
		params map[string]string
		want   string
	}{
		{
			name:   "no params, plain path",
			path:   "file:test.db",
			params: map[string]string{},
			want:   "file:test.db",
		},
		{
			name: "single param, plain path",
			path: "file:test.db",
			params: map[string]string{
				"_foreign_keys": "on",
			},
			want: "file:test.db?_foreign_keys=on",
		},
		{
			name: "multiple params, plain path",
			path: "file:test.db",
			params: map[string]string{
				"_foreign_keys": "on",
				"_journal_mode": "WAL",
			},
			// Note: map iteration order is random, so we can’t guarantee ordering.
			// Instead, we’ll check with Contains in assertions.
			want: "",
		},
		{
			name: "path with existing query",
			path: "file:test.db?mode=memory&cache=shared",
			params: map[string]string{
				"_foreign_keys": "on",
			},
			want: "file:test.db?mode=memory&cache=shared&_foreign_keys=on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := buildSQLiteDSN(tt.path, tt.params)

			if tt.want != "" {
				if got != tt.want {
					t.Errorf("got %q, want %q", got, tt.want)
				}
			} else {
				// For multi-param tests where order is not guaranteed,
				// check all expected substrings.
				for k, v := range tt.params {
					needle := k + "=" + v
					if !strings.Contains(got, needle) {
						t.Errorf("got %q, missing expected param %q", got, needle)
					}
				}
			}
		})
	}
}
