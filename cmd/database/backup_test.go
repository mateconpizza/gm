package database

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/db"
)

func TestNewBackup_Fails_If_DB_Does_Not_Exist(t *testing.T) {
	d := testutil.SetupDeps(t)
	d.Cfg.DBPath = filepath.Join(t.TempDir(), "nonexistent.db")

	err := backupNewFunc(d)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, db.ErrDBNotFound) {
		t.Fatalf("expected db.ErrDBNotFound, got %v", err)
	}
}

func TestNewBackup_Fails_If_DB_Is_Empty(t *testing.T) {
	d := testutil.SetupDeps(t)
	f, err := os.Create(d.Cfg.DBPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Errorf("unexpected err closing file: %v", err)
	}

	err = backupNewFunc(d)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, db.ErrDBEmpty) {
		t.Fatalf("expected db.ErrDBEmpty, got %v", err)
	}
}

func TestNewBackup_Successfully_Created(t *testing.T) {
	d := testutil.SetupDeps(t)

	d.Cfg.Path.Backup = filepath.Join(d.Cfg.Path.Data, "backup")
	d.Cfg.Flags.Yes = true
	d.Cfg.Flags.Force = true

	r := testutil.SetupInitializedDBWithBookmarks(t, d.Cfg.DBPath, 5)
	d.SetDatabase(r)

	var buf bytes.Buffer
	d.SetWriter(&buf)

	err := backupNewFunc(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(d.Cfg.Path.Backup)
	if err != nil {
		t.Fatalf("expected backup dir, got error: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected backup dir, got file")
	}

	output := buf.String()
	expectedString := "backup created:"
	if !strings.Contains(output, expectedString) {
		t.Errorf("want %q, got %q", expectedString, output)
	}
}

func TestNewBackup_Do_Not_ConfirmErr(t *testing.T) {
	d := testutil.SetupDeps(t)
	r := testutil.SetupInitializedDBWithBookmarks(t, d.Cfg.DBPath, 5)
	d.SetDatabase(r)

	// Update terminal for reject confirmation prompt.
	input := "n\n"
	term := terminal.New(terminal.WithContext(t.Context()), terminal.WithReader(strings.NewReader(input)))
	c := ui.NewConsole(ui.WithTerminal(term))
	d.SetConsole(c)

	err := backupNewFunc(d)
	if !errors.Is(err, sys.ErrExitFailure) {
		t.Fatalf("expected err %q, got %q", sys.ErrExitFailure, err)
	}
}
