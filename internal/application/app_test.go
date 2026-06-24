package application_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/testutil"
)

func TestAppValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) *application.App
		wantErr error
	}{
		{
			"valid_config",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Database = filepath.Join(t.TempDir(), app.DBName)
				return app
			},
			nil,
		},
		{
			"missing_db_name",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Database = filepath.Join(t.TempDir(), app.DBName)
				app.DBName = ""
				return app
			},
			application.ErrDatabaseNameNotSet,
		},
		{
			"db_name_only_suffixes",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Database = filepath.Join(t.TempDir(), app.DBName)
				app.DBName = ".db"
				return app
			},
			application.ErrDatabaseInvalidName,
		},
		{
			"missing_db_path",
			func(t *testing.T) *application.App {
				t.Helper()
				return testutil.SetupApp(t) // Path.Database is empty by default
			},
			application.ErrDatabasePathNotSet,
		},
		{
			"db_name_priority_over_path",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.DBName = ""
				// Path.Database also empty — DBName error should win
				return app
			},
			application.ErrDatabaseNameNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.setup(t).Validate()
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Validate() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Validate() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestAppSetDatabasePath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		input      string
		setup      func(t *testing.T) *application.App
		wantErr    error
		wantDBName string
	}{
		{
			"valid_name_no_extension",
			"mydb",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Data = t.TempDir()
				return app
			},
			nil,
			"mydb.db",
		},
		{
			"valid_name_with_db_extension",
			"mydb.db",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Data = t.TempDir()
				return app
			},
			nil,
			"mydb.db",
		},
		{
			"strips_extra_suffixes",
			"mydb.tar.gz",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Data = t.TempDir()
				return app
			},
			nil,
			"mydb.db",
		},
		{
			"empty_name_returns_error",
			"",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Data = t.TempDir()
				return app
			},
			application.ErrDatabaseNameNotSet,
			"",
		},
		{
			"only_suffixes_returns_error",
			".db",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.SetupApp(t)
				app.Path.Data = t.TempDir()
				return app
			},
			application.ErrDatabaseNameNotSet,
			"",
		},
		{
			"empty_data_path_returns_error",
			"mydb",
			func(t *testing.T) *application.App {
				t.Helper()
				return testutil.SetupApp(t) // Path.Data is empty by default
			},
			application.ErrDatabasePathNotSet,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := tt.setup(t)
			err := app.SetDatabase(tt.input)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("SetDatabasePath(%q) expected error %v, got nil", tt.input, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("SetDatabasePath(%q) expected error %v, got %v", tt.input, tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("SetDatabasePath(%q) unexpected error: %v", tt.input, err)
			}

			if app.DBName != tt.wantDBName {
				t.Errorf("DBName = %q; want %q", app.DBName, tt.wantDBName)
			}

			wantPath := filepath.Join(app.Path.Home(), tt.wantDBName)
			if app.Path.DB() != wantPath {
				t.Errorf("Path.Database = %q; want %q", app.Path.DB(), wantPath)
			}
		})
	}
}

func TestApp_GitEnabled(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		git  *application.Git
		want bool
	}{
		{"git_enabled", &application.Git{Enabled: true}, true},
		{"git_disabled", &application.Git{Enabled: false}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := testutil.SetupApp(t)
			app.Git = tt.git
			got := app.GitEnabled()
			if got != tt.want {
				t.Fatalf("App.GitEnabled() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestApp_Version(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info *application.Information
		want string
	}{
		{"normal_version", &application.Information{Version: "1.0.0"}, "1.0.0"},
		{"empty_version", &application.Information{Version: ""}, ""},
		{"pre_release_version", &application.Information{Version: "2.1.0-alpha"}, "2.1.0-alpha"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := testutil.SetupApp(t)
			app.Info = tt.info
			got := app.Version()
			if got != tt.want {
				t.Fatalf("App.Version() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestColorEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		colorStr        string
		stdinPiped      bool
		stdoutPiped     bool
		noColor         bool
		expectedEnabled bool
	}{
		{"always", "always", true, true, true, true},
		{"never", "never", false, false, false, false},
		{"auto interactive terminal", "auto", false, false, false, true},
		{"auto stdin piped", "auto", true, false, false, false},
		{"auto stdout piped", "auto", false, true, false, false},
		{"auto NO_COLOR set", "auto", false, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := application.ColorEnabled(
				tt.colorStr,
				func() bool { return tt.stdinPiped },
				func() bool { return tt.stdoutPiped },
				func() bool { return tt.noColor },
			)
			if result != tt.expectedEnabled {
				t.Errorf("got %v, want %v", result, tt.expectedEnabled)
			}
		})
	}
}
