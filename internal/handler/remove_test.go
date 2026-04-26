package handler

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func testSetupDBFiles(t *testing.T, tempDir string, n int) []string {
	t.Helper()
	r := make([]string, 0, n)

	for range n {
		tf, err := os.CreateTemp(tempDir, "sqlite-*.db")
		if err != nil {
			t.Fatal(err)
		}

		r = append(r, tf.Name())
	}

	return r
}

func TestRemoveRepo(t *testing.T) {
	t.Skip("skipping for now")
	t.Parallel()
	fs := testSetupDBFiles(t, t.TempDir(), 10)
	_ = fs
}

func TestRemoveBackups(t *testing.T) {
	t.Parallel()
	t.Skip("skipping for now")
}

func TestDatabase_Drop(t *testing.T) {
	d := testutil.SetupDeps(t)
	want := 10
	r := testutil.SetupInitializedDBWithBookmarks(t, d.App.Path.Database, want)
	d.SetRepo(r)
	c := testutil.ConsoleWithInput(t, "y\n")
	d.SetConsole(c)

	got, err := r.All(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != want {
		t.Fatalf("expected %d bookmarks, got: %d", want, len(got))
	}

	err = DropDatabase(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err = r.All(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("expected 0 bookmarks, got: %d", len(got))
	}
}

func TestRemoveRepo_Success(t *testing.T) {
	ansi.DisableColor()

	t.Run("successfully remove main database", func(t *testing.T) {
		d := testutil.SetupDeps(t)
		d.App.Flags.Force = true
		r := testutil.SetupInitializedEmptyDB(t, d.App.Path.Database)
		d.SetRepo(r)
		var buf bytes.Buffer
		d.SetWriter(&buf)

		err := RemoveRepo(d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Successfully database main removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(d.App.Path.Database) {
			t.Fatalf("file %q was not deleted", d.App.Path.Database)
		}
	})

	t.Run("successfully remove a database", func(t *testing.T) {
		d := testutil.SetupDeps(t)
		d.App.DBName = "somedatabase.db"
		d.App.Path.Database = filepath.Join(d.App.Path.Data, d.App.DBName)
		d.App.Flags.Force = true
		r := testutil.SetupInitializedEmptyDB(t, d.App.Path.Database)
		d.SetRepo(r)
		var buf bytes.Buffer
		d.SetWriter(&buf)

		err := RemoveRepo(d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Successfully database "+d.App.DBName+" removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(d.App.Path.Database) {
			t.Fatalf("file %q was not deleted", d.App.Path.Database)
		}
	})
}

func TestRemoveRepo_Fail(t *testing.T) {
	t.Run("fails with database not found", func(t *testing.T) {
		d := testutil.SetupDeps(t)
		d.App.Path.Database = filepath.Join(d.App.Path.Data, "nonexistent.db")

		err := RemoveRepo(d)
		if !errors.Is(err, db.ErrDBNotFound) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails with main database cannot be removed without flag force", func(t *testing.T) {
		d := testutil.SetupDeps(t)
		r := testutil.SetupInitializedEmptyDB(t, d.App.Path.Database)
		d.SetRepo(r)

		err := RemoveRepo(d)
		if !errors.Is(err, ErrInvalidOption) {
			t.Fatalf("unexpected error: %v", err)
		}

		gotOutput := err.Error()
		wantOutput := "main database cannot be removed"
		if !strings.Contains(gotOutput, wantOutput) {
			t.Fatalf("want: %q, got: %q", wantOutput, gotOutput)
		}
	})
}
