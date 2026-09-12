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
				app := testutil.NewApp(t)
				app.Path.Database = filepath.Join(t.TempDir(), app.DBName)
				return app
			},
			nil,
		},
		{
			"missing_db_name",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.NewApp(t)
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
				app := testutil.NewApp(t)
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
				return testutil.NewApp(t) // Path.Database is empty by default
			},
			application.ErrDatabasePathNotSet,
		},
		{
			"db_name_priority_over_path",
			func(t *testing.T) *application.App {
				t.Helper()
				app := testutil.NewApp(t)
				app.DBName = ""
				// Path.Database also empty - DBName error should win
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
			app := testutil.NewApp(t)
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
			app := testutil.NewApp(t)
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

func TestApp_Setup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		app     *application.App
		wantErr error
	}{
		{
			name: "normal_setup",
			app: &application.App{
				Name:   "myapp",
				DBName: "main.db",
				Env:    &application.Env{Home: "/home/user"},
				Path:   &application.Path{},
			},
			wantErr: nil,
		},
		{
			name: "empty_home_directory_uses_system_default",
			app: &application.App{
				Name:   "myapp",
				DBName: "main.db",
				Env:    &application.Env{Home: ""}, // gap.NewScope(gap.User, appName) resolves the OS default automatically,
				Path:   &application.Path{},
			},
			wantErr: nil,
		},
		{
			name: "empty_db_name_edge_case",
			app: &application.App{
				Name:   "myapp",
				DBName: "",
				Env:    &application.Env{Home: "/home/user"},
				Path:   &application.Path{},
			},
			wantErr: application.ErrDatabaseNameNotSet,
		},
		{
			name: "empty_app_name_boundary",
			app: &application.App{
				Name:   "",
				DBName: "main.db",
				Env:    &application.Env{Home: "/home/user"},
				Path:   &application.Path{},
			},
			wantErr: nil,
		},
		{
			name: "nested_database_name_boundary",
			app: &application.App{
				Name:   "myapp",
				DBName: "subdir/nested.db",
				Env:    &application.Env{Home: "/home/user"},
				Path:   &application.Path{},
			},
			wantErr: nil,
		},
		{
			name: "special_characters_in_name",
			app: &application.App{
				Name:   "my-app_1.0!",
				DBName: "main.db",
				Env:    &application.Env{Home: "/home/user"},
				Path:   &application.Path{},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.app.Setup()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("(*App).Setup() expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("(*App).Setup() expected error %q, got %q", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("(*App).Setup() unexpected error: %v", err)
			}

			// Verify that the side effect (setting app.Path.Data) occurred
			if tt.app.Path.Data == "" {
				t.Errorf("(*App).Setup() expected app.Path.Data to be populated, got empty string")
			}
		})
	}
}

func TestApp_SetDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dbNameInput string
		dataDir     string
		wantDBName  string
		wantDBPath  string
		wantErr     error
	}{
		{
			name:        "normal_typical_input",
			dbNameInput: "bookmarks",
			dataDir:     "/home/user/.local/share/app",
			wantDBName:  "bookmarks.db",
			wantDBPath:  filepath.Join("/home/user/.local/share/app", "bookmarks.db"),
			wantErr:     nil,
		},
		{
			name:        "normal_with_existing_extension",
			dbNameInput: "data.db",
			dataDir:     "/opt/app/data",
			wantDBName:  "data.db",
			wantDBPath:  filepath.Join("/opt/app/data", "data.db"),
			wantErr:     nil,
		},
		{
			name:        "empty_db_name_edge_case",
			dbNameInput: "",
			dataDir:     "/var/lib/app",
			wantDBName:  "",
			wantDBPath:  "",
			wantErr:     application.ErrDatabaseNameNotSet,
		},
		{
			name:        "empty_data_path_boundary",
			dbNameInput: "main",
			dataDir:     "",
			wantDBName:  "main.db",
			wantDBPath:  "",
			wantErr:     application.ErrDatabasePathNotSet,
		},
		{
			// StripExts removes all extensions in a loop
			name:        "multiple_extensions_stripped_boundary",
			dbNameInput: "my-app.tar.gz",
			dataDir:     "/tmp/data",
			wantDBName:  "my-app.db",
			wantDBPath:  filepath.Join("/tmp/data", "my-app.db"),
			wantErr:     nil,
		},
		{
			// Changed from my-app_1.0 to avoid .0 being stripped by StripExts
			name:        "special_characters_in_name",
			dbNameInput: "my-app_1!@",
			dataDir:     "/tmp/data",
			wantDBName:  "my-app_1!@.db",
			wantDBPath:  filepath.Join("/tmp/data", "my-app_1!@.db"),
			wantErr:     nil,
		},
		{
			name:        "whitespace_in_db_name",
			dbNameInput: "my db",
			dataDir:     "/data/store",
			wantDBName:  "my db.db",
			wantDBPath:  filepath.Join("/data/store", "my db.db"),
			wantErr:     nil,
		},
		{
			name:        "only_suffixes_returns_error",
			dbNameInput: ".db",
			dataDir:     "/data/store",
			wantDBName:  "",
			wantDBPath:  "",
			wantErr:     application.ErrDatabaseNameNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := application.NewApp(tt.dataDir)
			err := app.SetDatabase(tt.dbNameInput)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("(*App).SetDatabase(%q) expected error %v, got nil", tt.dbNameInput, tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("(*App).SetDatabase(%q) expected error %q, got %q", tt.dbNameInput, tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("(*App).SetDatabase(%q) unexpected error: %v", tt.dbNameInput, err)
			}

			if app.DBName != tt.wantDBName {
				t.Errorf("(*App).SetDatabase() DBName = %v, want %v", app.DBName, tt.wantDBName)
			}
			if app.Path.Database != tt.wantDBPath {
				t.Errorf("(*App).SetDatabase() Path.Database = %v, want %v", app.Path.Database, tt.wantDBPath)
			}
		})
	}
}
