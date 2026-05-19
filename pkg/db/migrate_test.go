package db

import "testing"

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
	ctx := t.Context()
	r := setupTestDB(t)
	defer teardownthewall(r.DB)

	if err := Migrate(ctx, r); err != nil {
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
