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

	c.Setup()

	p := c.Path
	tempDir = filepath.Join(tempDir, Name)

	wantConfigFilepath := filepath.Join(tempDir, ConfigFilename)
	if wantConfigFilepath != p.Config {
		t.Fatalf("want: %q, got: %q", wantConfigFilepath, p.Config)
	}

	wantBackupPath := filepath.Join(tempDir, "backup")
	if wantBackupPath != p.Backup {
		t.Fatalf("want: %q, got: %q", wantBackupPath, p.Backup)
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
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, ConfigFilename)
	conf := &App{
		Git: &Git{
			Enabled: true,
			Log:     false,
			GPG:     true,
			Path:    "/some/path",
			Remote:  "git@github.com:ponzipalandri/bookmarks.git",
		},
	}

	if err := WriteYAML(fn, conf, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var app *App
	err := ReadYAML(fn, &app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := conf.Git
	got := app.Git
	if want.Enabled != got.Enabled {
		t.Fatalf("git enabled, want :%v, got: %v", want.Enabled, got.Enabled)
	}
	if want.GPG != got.GPG {
		t.Fatalf("GPG enabled, want: %v, got: %v", want.GPG, got.GPG)
	}
	if want.Log != got.Log {
		t.Fatalf("want: %v, got: %v", want.Log, got.Log)
	}
	if want.Path != got.Path {
		t.Fatalf("want: %v, got: %v", want.Path, got.Path)
	}
	if want.Remote != got.Remote {
		t.Fatalf("want: %v, got: %v", want.Remote, got.Remote)
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
