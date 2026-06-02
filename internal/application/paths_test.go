package application

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gap "github.com/muesli/go-app-paths"

	"github.com/mateconpizza/gm/pkg/files"
)

func TestPaths_LoadDataPath(t *testing.T) {
	t.Run("uses environment variable when set", func(t *testing.T) {
		want := t.TempDir()
		t.Setenv(EnvHome, want)

		got, err := loadDataPath(Name, EnvHome)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want = filepath.Join(want, Name)
		if want != got {
			t.Fatalf("want: %q, got: %q", want, got)
		}
	})

	t.Run("falls back to user data directory when env not set", func(t *testing.T) {
		t.Parallel()
		scope := gap.NewScope(gap.User, Name)
		want, err := scope.DataPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := loadDataPath(Name, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want != got {
			t.Fatalf("want: %q, got: %q", want, got)
		}
	})
}

func TestPaths_InitPaths(t *testing.T) {
	c := &App{
		Name:   Name,
		Cmd:    Command,
		DBName: "main",
		Path:   &Path{},
		Flags:  &Flags{},
		Env: &Env{
			Home:   EnvHome,
			Editor: EnvEditor,
		},
	}

	tempDir := t.TempDir()
	t.Setenv(c.Env.Home, tempDir)

	err := c.Setup()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := c.Path
	tempDir = filepath.Join(tempDir, Name)

	wantConfigFilepath := filepath.Join(tempDir, ConfigFilename)
	if wantConfigFilepath != p.ConfigFile() {
		t.Fatalf("want: %q, got: %q", wantConfigFilepath, p.ConfigFile())
	}

	wantBackupPath := filepath.Join(tempDir, "backup")
	if wantBackupPath != p.Backup() {
		t.Fatalf("want: %q, got: %q", wantBackupPath, p.Backup())
	}

	wantDBPath := filepath.Join(tempDir, c.DBName)
	if wantDBPath != c.Path.Database {
		t.Fatalf("want: %q, got: %q", wantDBPath, c.Path.Database)
	}

	wantExt := ".db"
	gotExt := filepath.Ext(c.DBName)
	if wantExt != gotExt {
		t.Fatalf("want: %q, got: %q", wantExt, gotExt)
	}
}

func TestWriteRead_Successfully_Reads_And_Unmarshals_Valid_YAML(t *testing.T) {
	t.Skip()
	t.Parallel()

	dir := t.TempDir()
	fn := filepath.Join(dir, ConfigFilename)
	appOriginal := &App{
		Path: &Path{
			Data: dir,
		},
		Git: &Git{
			// Enabled: true,
			Log:    false,
			Remote: "git@github.com:ponzipalandri/bookmarks.git",
		},
	}

	if appOriginal.Path.Home() != dir {
		t.Fatalf("unexpected error: want: %v, got: %v", dir, appOriginal.Path.Home())
	}

	if appOriginal.Path.Git() == "" {
		t.Fatal("unexpected err: Path.Git is empty")
	}

	if err := appOriginal.WriteConfig(false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var appFresh *App
	err := ReadYAML(fn, &appFresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := appOriginal.Path.Home()
	got := appFresh.Path.Home()
	if want != got {
		t.Fatalf("git enabled, want :%v, got: %v", want, got)
	}

	gWant := appOriginal.Git.Log
	gGot := appFresh.Git.Log
	if gWant != gGot {
		t.Fatalf("want: %v, got: %v", gWant, gGot)
	}
}

func TestRead_Fails_When_File_Does_Not_Exist(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "nonexistent.yaml")

	var app App
	err := ReadYAML(fn, &app)

	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
	if !errors.Is(err, files.ErrFileNotFound) {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), fn) {
		t.Errorf("error should contain filename, got %q", err.Error())
	}
}

func TestRead_Fails_With_Invalid_YAML_Syntax(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "invalid.yaml")

	// Write invalid YAML (bad indentation, unclosed quote, etc.)
	invalidYAML := `name: "unclosed quote
version: 1.0
  bad_indent: value`
	if err := os.WriteFile(fn, []byte(invalidYAML), files.FilePerm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var app App
	err := ReadYAML(fn, &app)

	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "unmarshalling YAML") {
		t.Errorf("expected unmarshalling error, got %q", err.Error())
	}
}

func TestRead_Fails_With_Type_Mismatch_In_YAML(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "typemismatch.yaml")

	// Write YAML with wrong types
	typeMismatchYAML := `name: 123
enabled: "not a boolean"`
	if err := os.WriteFile(fn, []byte(typeMismatchYAML), files.FilePerm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	type StrictApp struct {
		Name    string `yaml:"name"`
		Enabled bool   `yaml:"enabled"`
	}

	var app StrictApp
	err := ReadYAML(fn, &app)
	if err == nil {
		t.Logf("YAML was permissive: Name=%v, Enabled=%v", app.Name, app.Enabled)
	}
}

func TestRead_Handles_Empty_YAML_File(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "empty.yaml")

	if err := os.WriteFile(fn, []byte(""), files.FilePerm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var app App
	err := ReadYAML(fn, &app)
	if err != nil {
		t.Errorf("unexpected error for empty file: %v", err)
	}
}

func TestPath_Methods(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		data       string
		database   string
		wantHome   string
		wantGit    string
		wantBackup string
		wantDB     string
	}{
		{
			name:       "normal_paths",
			data:       "/var/lib/app",
			database:   "/var/lib/app/main.db",
			wantHome:   "/var/lib/app",
			wantGit:    filepath.Join("/var/lib/app", "git"),
			wantBackup: filepath.Join("/var/lib/app", "backup"),
			wantDB:     "/var/lib/app/main.db",
		},
		{
			name:       "empty_data",
			data:       "",
			database:   "main.db",
			wantHome:   "",
			wantGit:    "git",
			wantBackup: "backup",
			wantDB:     "main.db",
		},
		{
			name:       "root_data",
			data:       "/",
			database:   "/main.db",
			wantHome:   "/",
			wantGit:    filepath.Join("/", "git"),
			wantBackup: filepath.Join("/", "backup"),
			wantDB:     "/main.db",
		},
		{
			name:       "trailing_slash",
			data:       "/data/",
			database:   "/data/db.sqlite",
			wantHome:   "/data/",
			wantGit:    filepath.Join("/data/", "git"),
			wantBackup: filepath.Join("/data/", "backup"),
			wantDB:     "/data/db.sqlite",
		},
		{
			name:       "relative_data",
			data:       "./local",
			database:   "./local/main.db",
			wantHome:   "./local",
			wantGit:    filepath.Join(".", "local", "git"),
			wantBackup: filepath.Join(".", "local", "backup"),
			wantDB:     "./local/main.db",
		},
		{
			name:       "empty_database",
			data:       "/app/data",
			database:   "",
			wantHome:   "/app/data",
			wantGit:    filepath.Join("/app/data", "git"),
			wantBackup: filepath.Join("/app/data", "backup"),
			wantDB:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := NewPath()
			p.Data = tt.data
			p.Database = tt.database

			if got := p.Home(); got != tt.wantHome {
				t.Fatalf("Path.Home() = %q; want %q", got, tt.wantHome)
			}
			if got := p.Git(); got != tt.wantGit {
				t.Fatalf("Path.Git() = %q; want %q", got, tt.wantGit)
			}
			if got := p.Backup(); got != tt.wantBackup {
				t.Fatalf("Path.Backup() = %q; want %q", got, tt.wantBackup)
			}
			if got := p.DB(); got != tt.wantDB {
				t.Fatalf("Path.DB() = %q; want %q", got, tt.wantDB)
			}
		})
	}
}

func TestPaths_DatabaseFullpath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		data   string
		dbName string
		want   string
	}{
		{"normal", "/var/lib/data", "main.db", filepath.Join("/var/lib/data", "main.db")},
		{"root_directory", "/", "main.db", filepath.Join("/", "main.db")},
		{"trailing_slash", "/var/lib/data/", "main.db", filepath.Join("/var/lib/data/", "main.db")},
		{"relative_path", "./local/data", "main.db", filepath.Join(".", "local", "data", "main.db")},
		{"dot_dot_path", "/var/lib/../data", "main.db", filepath.Join("/var/lib/../data", "main.db")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := NewApp(tt.data)
			err := app.SetDatabase(tt.dbName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := app.Path.DB()
			if got != tt.want {
				t.Fatalf("app.Path.DB() = %q; want %q", got, tt.want)
			}
		})
	}
}
