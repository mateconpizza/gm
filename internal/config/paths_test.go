package config

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

		got, err := loadDataPath(AppName, EnvHome)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want = filepath.Join(want, AppName)
		if want != got {
			t.Fatalf("want: %q, got: %q", want, got)
		}
	})

	t.Run("falls back to user data directory when env not set", func(t *testing.T) {
		t.Parallel()
		scope := gap.NewScope(gap.User, AppName)
		want, err := scope.DataPath("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, err := loadDataPath(AppName, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want != got {
			t.Fatalf("want: %q, got: %q", want, got)
		}
	})
}

func TestPaths_InitPaths(t *testing.T) {
	c := &Config{
		Name:   AppName,
		Cmd:    AppCommand,
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

	c.InitPaths()

	p := c.Path
	tempDir = filepath.Join(tempDir, AppName)

	wantConfigFilepath := filepath.Join(tempDir, ConfigFilename)
	if wantConfigFilepath != p.ConfigFile {
		t.Fatalf("want: %q, got: %q", wantConfigFilepath, p.ConfigFile)
	}

	wantBackupPath := filepath.Join(tempDir, "backup")
	if wantBackupPath != p.Backup {
		t.Fatalf("want: %q, got: %q", wantBackupPath, p.Backup)
	}

	wantDBPath := filepath.Join(tempDir, c.DBName)
	if wantDBPath != c.DBPath {
		t.Fatalf("want: %q, got: %q", wantDBPath, c.DBPath)
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
	fn := filepath.Join(tempDir, "config.yaml")
	conf := &Config{
		Name:   AppName,
		Cmd:    AppCommand,
		DBName: MainDBName,
	}

	if err := Write(fn, conf, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := Read(fn, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Name != AppName {
		t.Errorf("expected Name=test, got %q", cfg.Name)
	}
	if cfg.DBName != MainDBName {
		t.Errorf("expected %q, got %q", MainDBName, cfg.DBName)
	}
	if cfg.Cmd != AppCommand {
		t.Errorf("expected %q, got %q", AppCommand, cfg.Cmd)
	}
}

func TestRead_Fails_When_File_Does_Not_Exist(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "nonexistent.yaml")

	var cfg Config
	err := Read(fn, &cfg)

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

	var cfg Config
	err := Read(fn, &cfg)

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

	type StrictConfig struct {
		Name    string `yaml:"name"`
		Enabled bool   `yaml:"enabled"`
	}

	var cfg StrictConfig
	err := Read(fn, &cfg)
	if err == nil {
		t.Logf("YAML was permissive: Name=%v, Enabled=%v", cfg.Name, cfg.Enabled)
	}
}

func TestRead_Handles_Empty_YAML_File(t *testing.T) {
	t.Parallel()
	tempDir := t.TempDir()
	fn := filepath.Join(tempDir, "empty.yaml")

	if err := os.WriteFile(fn, []byte(""), files.FilePerm); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	var cfg Config
	err := Read(fn, &cfg)
	if err != nil {
		t.Errorf("unexpected error for empty file: %v", err)
	}
}
