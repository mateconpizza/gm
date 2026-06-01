package config

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/files"
)

func TestConfig_Create(t *testing.T) {
	t.Parallel()

	t.Run("creates config file when user confirms", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		c := ui.NewConsole(
			ui.WithWriter(&buf),
			ui.WithTerminal(terminal.New(
				terminal.WithReader(strings.NewReader("y\n")), // input Yes when asking to create file
				terminal.WithWriter(io.Discard),               // send output to null, show no prompt
			)),
		)

		dir := t.TempDir()
		app := testutil.SetupApp(t)
		fn := application.ConfigFilename
		app.Path.Config = filepath.Join(dir, fn)

		err := createConfig(t.Context(), c, app)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, fn) {
			t.Errorf("output %q should contain filename %q", output, fn)
		}
	})

	t.Run("returns ErrActionAborted when user declines", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer

		c := ui.NewConsole(
			ui.WithWriter(&buf),
			ui.WithTerminal(terminal.New(
				terminal.WithReader(strings.NewReader("n\n")), // decline
				terminal.WithWriter(io.Discard),
			)),
		)

		app := testutil.SetupApp(t)
		app.Flags.Yes = true
		dir := t.TempDir()
		app.Path.Config = filepath.Join(dir, application.ConfigFilename)

		err := createConfig(t.Context(), c, app)
		if err == nil {
			t.Fatal("expected error but got none")
		}
		if !errors.Is(err, sys.ErrActionAborted) {
			t.Fatalf("got error %v, want %v", err, sys.ErrActionAborted)
		}
	})

	t.Run("returns ErrFileExists when config file already exists", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, application.ConfigFilename)
		_, err := files.Touch(path, false)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		app := testutil.SetupApp(t)
		app.Path.Data = dir

		c := ui.NewConsole()
		err = createConfig(t.Context(), c, app)
		if !errors.Is(err, files.ErrFileExists) {
			t.Fatalf("got error %v, want %v", err, files.ErrFileExists)
		}

		wantErrStr := "file already exists."
		if !strings.Contains(err.Error(), wantErrStr) {
			t.Errorf("error message %q should contain %q", err.Error(), wantErrStr)
		}

		want := path
		got := app.Path.ConfigFile()
		if want != got {
			t.Fatalf("unexpected path. want: %q, got: %q", want, got)
		}
	})
}
