package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/dbops"
	"github.com/mateconpizza/gm/internal/testutil"
)

func TestExtractIDsFromString(t *testing.T) {
	t.Parallel()

	t.Run("extract valid IDs", func(t *testing.T) {
		t.Parallel()
		idsStr := []string{"1", "2", "3"}
		ids, err := extractIDsFrom(idsStr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expected := []int{1, 2, 3}
		if !equalIntSlice(ids, expected) {
			t.Errorf("got %v, want %v", ids, expected)
		}
	})

	t.Run("invalid IDs", func(t *testing.T) {
		t.Parallel()
		nonIntStr := []string{"a", "b", "c"}
		ids, err := extractIDsFrom(nonIntStr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected empty slice, got %v", ids)
		}
	})
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestValidateRenameTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(app *application.App)
		wantErr error
	}{
		{
			name: "normal_valid_target",
			setup: func(app *application.App) {
				app.DBName = "user_data.db"
			},
			wantErr: nil,
		},
		{
			name: "main_database_exact_match",
			setup: func(app *application.App) {
				app.DBName = application.MainDBName
			},
			wantErr: dbops.ErrInvalidRename,
		},
		{
			name: "main_database_case_sensitive",
			setup: func(app *application.App) {
				// == comparison is case-sensitive, so this should not match the main DB
				app.DBName = strings.ToUpper(application.MainDBName)
			},
			wantErr: nil,
		},
		{
			name: "empty_dbname",
			setup: func(app *application.App) {
				app.DBName = ""
			},
			wantErr: nil,
		},
		{
			// Assuming DBBaseName() and DefaultDB() derive from DBName or match this naming convention.
			name: "default_database_exact_match",
			setup: func(app *application.App) {
				app.DBName = application.MainDBName
			},
			wantErr: dbops.ErrInvalidRename,
		},
		{
			name: "default_database_no_extension",
			setup: func(app *application.App) {
				// strip("main") == "main", matching strip("main.db")
				app.DBName = "main"
			},
			wantErr: dbops.ErrInvalidRename,
		},
		{
			name: "multiple_extensions_valid",
			setup: func(app *application.App) {
				app.DBName = "my_data.tar.gz"
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testutil.NewApp(t)
			tt.setup(app)

			err := ValidateRenameTarget(app)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ValidateRenameTarget() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateRenameTarget() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ValidateRenameTarget() unexpected error: %v", err)
			}
		})
	}
}
