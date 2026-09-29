package application_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui/formatter"
)

func TestApp_Validate(t *testing.T) {
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

func TestApp_Load(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T, tmpDir string) *application.App
		wantErr error
	}{
		{
			name: "normal_valid_config_file",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				content := []byte("db: main.db\ncmd: app\nui:\n  format: text\n  glyphs: ascii\n")
				testutil.NewFile(t, tmpDir, application.ConfigFilename, content)
				return &application.App{
					DBName: "main.db",
					Path: &application.Path{
						Data:     tmpDir,
						Database: filepath.Join(tmpDir, "main.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
			},
			wantErr: nil,
		},
		{
			name: "normal_config_not_exist_uses_defaults",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				return &application.App{
					DBName: "default.db",
					Path: &application.Path{
						Data:     tmpDir,
						Database: filepath.Join(tmpDir, "default.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
			},
			wantErr: nil,
		},
		{
			name: "normal_invalid_config_uses_defaults",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				content := []byte("invalid_yaml: [unclosed_list")
				testutil.NewFile(t, tmpDir, application.ConfigFilename, content)
				return &application.App{
					DBName: "default.db",
					Path: &application.Path{
						Data:     tmpDir,
						Database: filepath.Join(tmpDir, "default.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
			},
			wantErr: nil,
		},
		{
			name: "error_get_config_io_failure",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				unreadableDir := filepath.Join(tmpDir, "unreadable")
				if err := os.MkdirAll(unreadableDir, 0o000); err != nil {
					t.Fatalf("failed to create unreadable directory: %v", err)
				}
				return &application.App{
					Path: &application.Path{
						Data:     unreadableDir,
						Database: filepath.Join(unreadableDir, "default.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
			},
			wantErr: os.ErrPermission,
		},
		{
			name: "error_invalid_glyph_mode",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				app := &application.App{
					DBName: "default.db",
					Path: &application.Path{
						Data:     tmpDir,
						Database: filepath.Join(tmpDir, "default.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
				app.UI.GlyphMode = formatter.GlyphMode("invalid_mode")
				return app
			},
			wantErr: formatter.ErrGlyphModeInvalid,
		},
		{
			name: "boundary_empty_db_name",
			setup: func(t *testing.T, tmpDir string) *application.App {
				t.Helper()

				return &application.App{
					DBName: "",
					Path: &application.Path{
						Data:     tmpDir,
						Database: filepath.Join(tmpDir, "default.db"),
					},
					Flags: &application.Flags{},
					UI:    application.NewUI(),
				}
			},
			wantErr: application.ErrDatabaseNameNotSet,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			app := tt.setup(t, tmpDir)

			err := app.Load()

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Load() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Load() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Load() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Load() unexpected error: %v", err)
			}
		})
	}
}

func TestApp_PrettyVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		appName string
		version string
		color   bool
		verbose int
		commit  string
		date    string

		wantContains    []string // substrings that must appear
		wantNotContains []string // substrings that must NOT appear
	}{
		{
			name:         "normal_quiet_no_color",
			appName:      "gm",
			version:      "1.2.3",
			color:        false,
			verbose:      0,
			wantContains: []string{"gm", "v1.2.3", runtime.GOOS + "/" + runtime.GOARCH},
		},
		{
			name:            "dev_version_no_v_prefix",
			appName:         "gm",
			version:         "dev",
			color:           false,
			verbose:         0,
			wantContains:    []string{"gm", "dev", runtime.GOOS + "/" + runtime.GOARCH},
			wantNotContains: []string{"vdev"},
		},
		{
			name:         "empty_name_and_version",
			appName:      "",
			version:      "",
			color:        false,
			verbose:      0,
			wantContains: []string{"v", runtime.GOOS + "/" + runtime.GOARCH},
		},
		{
			name:         "color_enabled_wraps_name",
			appName:      "gm",
			version:      "1.0.0",
			color:        true,
			verbose:      0,
			wantContains: []string{"\x1b[", "gm", "v1.0.0"},
		},
		{
			name:         "verbose_shows_commit_and_date",
			appName:      "gm",
			version:      "1.2.3",
			color:        false,
			verbose:      1,
			commit:       "abc123",
			date:         "2026-01-01T00:00:00Z",
			wantContains: []string{"commit:", "abc123", "built:", "2026-01-01T00:00:00Z", "go version:", runtime.Version(), "platform:", runtime.GOOS + "/" + runtime.GOARCH},
		},
		{
			name:            "verbose_omits_none_commit",
			appName:         "gm",
			version:         "1.2.3",
			color:           false,
			verbose:         1,
			commit:          "none",
			date:            "2026-01-01T00:00:00Z",
			wantContains:    []string{"built:", "2026-01-01T00:00:00Z"},
			wantNotContains: []string{"commit:"},
		},
		{
			name:            "verbose_omits_unknown_date",
			appName:         "gm",
			version:         "1.2.3",
			color:           false,
			verbose:         1,
			commit:          "abc123",
			date:            "unknown",
			wantContains:    []string{"commit:", "abc123"},
			wantNotContains: []string{"built:"},
		},
		{
			name:            "verbose_omits_empty_commit_and_date",
			appName:         "gm",
			version:         "1.2.3",
			color:           false,
			verbose:         1,
			commit:          "",
			date:            "",
			wantContains:    []string{"go version:", runtime.Version(), "platform:", runtime.GOOS + "/" + runtime.GOARCH},
			wantNotContains: []string{"commit:", "built:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := &application.App{
				Name: tt.appName,
				Info: &application.Information{
					Version: tt.version,
					Commit:  tt.commit,
					Date:    tt.date,
				},
				Flags: &application.Flags{
					Color:   tt.color,
					Verbose: tt.verbose,
				},
			}

			got := app.PrettyVersion()

			for _, sub := range tt.wantContains {
				if !strings.Contains(got, sub) {
					t.Fatalf("PrettyVersion() = %q; want it to contain %q", got, sub)
				}
			}

			for _, sub := range tt.wantNotContains {
				if strings.Contains(got, sub) {
					t.Fatalf("PrettyVersion() = %q; want it to NOT contain %q", got, sub)
				}
			}

			// Sanity check: quiet mode is a single line, verbose mode spans multiple.
			if tt.verbose == 0 {
				if strings.Count(got, "\n") != 1 {
					t.Fatalf("PrettyVersion() quiet mode = %q; want exactly one newline", got)
				}
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
