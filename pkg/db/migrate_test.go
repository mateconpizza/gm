package db

import (
	"context"
	"errors"
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseMigrationFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		input         string
		wantVersion   int
		wantMigration string
		wantErr       bool
	}{
		{
			name:          "valid_filename",
			input:         "0001_init.sql",
			wantVersion:   1,
			wantMigration: "init",
			wantErr:       false,
		},
		{
			name:          "valid_filename_multiple_words",
			input:         "0010_add_metadata_table.sql",
			wantVersion:   10,
			wantMigration: "add_metadata_table",
			wantErr:       false,
		},
		{
			name:          "missing_separator",
			input:         "0001init.sql",
			wantVersion:   0,
			wantMigration: "",
			wantErr:       true,
		},
		{
			name:          "empty_string",
			input:         "",
			wantVersion:   0,
			wantMigration: "",
			wantErr:       true,
		},
		{
			name:          "invalid_version",
			input:         "abcd_create_table.sql",
			wantVersion:   0,
			wantMigration: "",
			wantErr:       true,
		},
		{
			name:          "zero_version",
			input:         "0000_bootstrap.sql",
			wantVersion:   0,
			wantMigration: "bootstrap",
			wantErr:       false,
		},
		{
			name:          "missing_sql_extension",
			input:         "0002_indexes",
			wantVersion:   2,
			wantMigration: "indexes",
			wantErr:       false,
		},
		{
			name:          "empty_migration_name",
			input:         "0003_.sql",
			wantVersion:   3,
			wantMigration: "",
			wantErr:       false,
		},
		{
			name:          "negative_version",
			input:         "-1_invalid.sql",
			wantVersion:   -1,
			wantMigration: "invalid",
			wantErr:       false,
		},
		{
			name:          "only_separator",
			input:         "_.sql",
			wantVersion:   0,
			wantMigration: "",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotVersion, gotMigration, err := parseMigrationFilename(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMigrationFilename(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseMigrationFilename(%q) unexpected error: %v", tt.input, err)
			}

			if gotVersion != tt.wantVersion {
				t.Fatalf(
					"parseMigrationFilename(%q) version = %d; want %d",
					tt.input,
					gotVersion,
					tt.wantVersion,
				)
			}

			if gotMigration != tt.wantMigration {
				t.Fatalf(
					"parseMigrationFilename(%q) migration = %q; want %q",
					tt.input,
					gotMigration,
					tt.wantMigration,
				)
			}
		})
	}
}

func TestMigrate_TablesExist(t *testing.T) {
	t.Parallel()
	r := setupTestDB(t)

	ms, err := LoadMigrations()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := Migrate(t.Context(), r, ms); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	tables := []Table{"bookmarks", "tags", "bookmark_tags", "metadata"}

	for _, table := range tables {
		ok, err := table.Exists(t.Context(), r)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !ok {
			t.Errorf("table %q not found after migration", table)
		}
	}
}

func TestLoadMigrations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fsys       fs.FS
		want       []Migration
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "valid_single_migration",
			fsys: fstest.MapFS{
				"migrations/0001_init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
			},
			want: []Migration{
				{
					Version: 1,
					Name:    "init",
					File:    "0001_init.sql",
					SQL:     "CREATE TABLE users(id INT);",
				},
			},
		},
		{
			name: "valid_multiple_migrations_sorted",
			fsys: fstest.MapFS{
				"migrations/0002_add_users.sql": {
					Data: []byte("ALTER TABLE users ADD COLUMN name TEXT;"),
				},
				"migrations/0001_init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
			},
			want: []Migration{
				{
					Version: 1,
					Name:    "init",
					File:    "0001_init.sql",
					SQL:     "CREATE TABLE users(id INT);",
				},
				{
					Version: 2,
					Name:    "add_users",
					File:    "0002_add_users.sql",
					SQL:     "ALTER TABLE users ADD COLUMN name TEXT;",
				},
			},
		},
		{
			name: "ignores_non_sql_files_and_directories",
			fsys: fstest.MapFS{
				"migrations/0001_init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
				"migrations/readme.txt": {
					Data: []byte("documentation"),
				},
				"migrations/.keep": {
					Data: []byte(""),
				},
				"migrations/subdir/0002_nested.sql": {
					Data: []byte("SHOULD NOT LOAD"),
				},
			},
			want: []Migration{
				{
					Version: 1,
					Name:    "init",
					File:    "0001_init.sql",
					SQL:     "CREATE TABLE users(id INT);",
				},
			},
		},
		{
			name: "empty_migrations_directory",
			fsys: fstest.MapFS{
				"migrations/.keep": {
					Data: []byte(""),
				},
			},
			want: []Migration{},
		},
		{
			name: "missing_migrations_directory",
			fsys: fstest.MapFS{
				"other/file.txt": {
					Data: []byte("content"),
				},
			},
			wantErr: fs.ErrNotExist,
		},
		{
			name: "invalid_filename_format",
			fsys: fstest.MapFS{
				"migrations/init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
			},
			wantErrMsg: "init.sql",
		},
		{
			name: "migration_gap",
			fsys: fstest.MapFS{
				"migrations/0001_init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
				"migrations/0003_add_users.sql": {
					Data: []byte("ALTER TABLE users ADD COLUMN name TEXT;"),
				},
			},
			wantErr: ErrMigrationGap,
		},
		{
			name: "duplicate_migration_versions",
			fsys: fstest.MapFS{
				"migrations/0001_init.sql": {
					Data: []byte("CREATE TABLE users(id INT);"),
				},
				"migrations/0001_duplicate.sql": {
					Data: []byte("CREATE TABLE posts(id INT);"),
				},
			},
			wantErr: ErrMigrationDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := loadMigrations(tt.fsys)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("loadMigrations() expected error %v, got nil", tt.wantErr)
				}

				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("loadMigrations() error = %v; want %v", err, tt.wantErr)
				}

				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("loadMigrations() expected error containing %q, got nil", tt.wantErrMsg)
				}

				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("loadMigrations() error = %q; want substring %q", err.Error(), tt.wantErrMsg)
				}

				return
			}

			if err != nil {
				t.Fatalf("loadMigrations() unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("loadMigrations() = %#v; want %#v", got, tt.want)
			}
		})
	}
}

func TestMigrate_Scenarios(t *testing.T) {
	t.Parallel()
	// valid migration slice to reuse
	sampleMigrations := []Migration{
		{Version: 1, Name: "create_users", SQL: "CREATE TABLE users (id INTEGER PRIMARY KEY);"},
		{Version: 2, Name: "add_email", SQL: "ALTER TABLE users ADD COLUMN email TEXT;"},
	}

	tests := []struct {
		name        string
		noMigration bool                                 // If true, starts with a completely raw, unmigrated DB
		setupFn     func(ctx context.Context, r *SQLite) // Setup custom starting states
		migrations  []Migration
		wantErr     bool
		wantErrMsg  string
	}{
		{
			name:        "normal_fresh_database_migration_success",
			noMigration: true,
			migrations:  sampleMigrations,
			wantErr:     false,
		},
		{
			name:        "boundary_already_at_latest_version_skips",
			noMigration: false,
			migrations:  sampleMigrations,
			wantErr:     false,
		},
		{
			name:        "empty_migrations_slice_noop",
			noMigration: true,
			migrations:  []Migration{},
			wantErr:     false,
		},
		{
			name:        "nil_migrations_slice_noop",
			noMigration: true,
			migrations:  nil,
			wantErr:     false,
		},
		{
			name:        "boundary_partial_migrations_applied_runs_pending",
			noMigration: true,
			setupFn: func(ctx context.Context, r *SQLite) {
				// manually apply just step 1 ahead of time
				_, err := r.DB.ExecContext(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY); PRAGMA user_version = 1;")
				if err != nil {
					panic(err)
				}
			},
			migrations: sampleMigrations,
			wantErr:    false,
		},
		{
			name:        "error_path_invalid_sql_syntax_rolls_back",
			noMigration: true,
			migrations: []Migration{
				{Version: 1, Name: "broken_sql", SQL: "CREATE TABLE IS NOT VALID SYNTAX;"},
			},
			wantErr:    true,
			wantErrMsg: "apply migration 0001_broken_sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var r *SQLite
			if tt.noMigration {
				r = setupTestDBNoMigration(t)
			} else {
				r = setupTestDB(t)
			}

			ctx := t.Context()

			if tt.setupFn != nil {
				tt.setupFn(ctx, r)
			}

			err := Migrate(ctx, r, tt.migrations)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Migrate() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("Migrate() error = %v; want error containing %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("Migrate() unexpected error: %v", err)
			}

			// post-condition validation for successful steps
			if !tt.wantErr && len(tt.migrations) > 0 {
				current, err := CurrentSchemaVersion(ctx, r)
				if err != nil {
					t.Fatalf("failed to check post-migration schema version: %v", err)
				}

				expectedVersion := tt.migrations[len(tt.migrations)-1].Version
				if current < expectedVersion {
					t.Errorf("expected final database version to be at least %d, got %d", expectedVersion, current)
				}
			}
		})
	}
}
