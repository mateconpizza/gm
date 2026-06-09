package dbops

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func TestDatabase_Drop(t *testing.T) {
	t.Parallel()
	d := testutil.SetupDeps(t)
	want := 10
	app, err := d.Application(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := testutil.SetupInitializedDBWithBookmarks(t, app.Path.DB(), want)
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

	err = Drop(t.Context(), d)
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
	t.Parallel()
	ansi.DisableColor()

	t.Run("successfully remove main database", func(t *testing.T) {
		t.Parallel()
		d := testutil.SetupDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Flags.Force = true
		r := testutil.SetupInitializedEmptyDB(t, app.Path.DB())
		d.SetRepo(r)
		var buf bytes.Buffer
		d.SetWriter(&buf)

		err = Remove(t.Context(), d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Successfully database main removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(app.Path.DB()) {
			t.Fatalf("file %q was not deleted", app.Path.DB())
		}
	})

	t.Run("successfully remove a database", func(t *testing.T) {
		t.Parallel()
		d := testutil.SetupDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.DBName = "somedatabase.db"
		app.Path.Database = filepath.Join(app.Path.Data, app.DBName)
		app.Flags.Force = true
		r := testutil.SetupInitializedEmptyDB(t, app.Path.DB())
		d.SetRepo(r)
		var buf bytes.Buffer
		d.SetWriter(&buf)

		err = Remove(t.Context(), d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		name := files.StripSuffixes(app.DBName)
		if !strings.Contains(output, "Successfully database "+name+" removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(app.Path.DB()) {
			t.Fatalf("file %q was not deleted", app.Path.DB())
		}
	})
}

func TestRemoveRepo_Fail(t *testing.T) {
	t.Parallel()

	t.Run("fails with database not found", func(t *testing.T) {
		t.Parallel()
		d := testutil.SetupDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Path.Database = filepath.Join(app.Path.Data, "nonexistent.db")

		err = Remove(t.Context(), d)
		if !errors.Is(err, db.ErrDBNotFound) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails with main database cannot be removed without flag force", func(t *testing.T) {
		t.Parallel()
		d := testutil.SetupDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := testutil.SetupInitializedEmptyDB(t, app.Path.DB())
		d.SetRepo(r)

		err = Remove(t.Context(), d)
		if !errors.Is(err, ErrInvalidOption) {
			t.Fatalf("unexpected error: %v", err)
		}

		gotOutput := err.Error()
		wantOutput := "removing the main database requires"
		if !strings.Contains(gotOutput, wantOutput) {
			t.Fatalf("want: %q, got: %q", wantOutput, gotOutput)
		}
	})
}

func TestPasswordInput(t *testing.T) {
	t.Run("valid password input", func(t *testing.T) {
		t.Parallel()
		pwd := "123"
		input := strings.NewReader(pwd + "\n" + pwd + "\n")

		c := ui.NewConsole(
			ui.WithFrame(frame.New()),
			ui.WithTerminal(terminal.New(
				terminal.WithWriter(io.Discard),
				terminal.WithReader(input),
			)),
		)

		s, err := passwordConfirm(t.Context(), c)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if s != pwd {
			t.Errorf("got %q, want %q", s, pwd)
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		t.Parallel()
		input := strings.NewReader("password1\npassword2\n")
		c := ui.NewConsole(
			ui.WithFrame(frame.New()),
			ui.WithTerminal(terminal.New(
				terminal.WithWriter(io.Discard),
				terminal.WithReader(input),
			)),
		)

		s, err := passwordConfirm(t.Context(), c)
		if err == nil {
			t.Error("expected error, got none")
		}
		if !errors.Is(err, locker.ErrPassphraseMismatch) {
			t.Errorf("expected ErrPassphraseMismatch, got %v", err)
		}
		if s != "" {
			t.Errorf("expected empty string, got %q", s)
		}
	})
}

func TestNewBackup_Fails_If_DB_Does_Not_Exist(t *testing.T) {
	t.Parallel()
	d := testutil.SetupDeps(t)
	app, err := d.Application(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	app.Path.Database = filepath.Join(t.TempDir(), "nonexistent.db")

	err = NewBackup(t.Context(), d)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, db.ErrDBNotFound) {
		t.Fatalf("expected db.ErrDBNotFound, got %v", err)
	}
}

func TestNewBackup_Fails_If_DB_Is_Empty(t *testing.T) {
	t.Parallel()
	d := testutil.SetupDeps(t)
	app, err := d.Application(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f, err := os.Create(app.Path.DB())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := f.Close(); err != nil {
		t.Errorf("unexpected err closing file: %v", err)
	}

	err = NewBackup(t.Context(), d)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, db.ErrDBEmpty) {
		t.Fatalf("expected db.ErrDBEmpty, got %v", err)
	}
}

func TestNewBackup_Successfully_Created(t *testing.T) {
	t.Parallel()
	d := testutil.SetupDeps(t)
	app, err := d.Application(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	app.Flags.Yes = true
	app.Flags.Force = true

	r := testutil.SetupInitializedDBWithBookmarks(t, app.Path.DB(), 5)
	d.SetRepo(r)

	var buf bytes.Buffer
	d.SetWriter(&buf)

	err = NewBackup(t.Context(), d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(app.Path.Backup())
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
	t.Parallel()
	d := testutil.SetupDeps(t)
	app, err := d.Application(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := testutil.SetupInitializedDBWithBookmarks(t, app.Path.DB(), 5)
	d.SetRepo(r)

	// Update terminal for reject confirmation prompt.
	input := "n\n"
	term := terminal.New(terminal.WithReader(strings.NewReader(input)))
	c := ui.NewConsole(ui.WithTerminal(term))
	d.SetConsole(c)

	err = NewBackup(t.Context(), d)
	if !errors.Is(err, sys.ErrExitFailure) {
		t.Fatalf("expected err %q, got %q", sys.ErrExitFailure, err)
	}
}
