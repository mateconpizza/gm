package dbops

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

func TestDatabase_Drop(t *testing.T) {
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
	ansi.DisableColor()

	t.Run("successfully remove main database", func(t *testing.T) {
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
	t.Run("fails with database not found", func(t *testing.T) {
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
