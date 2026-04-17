package appcfg

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
		fn := "config.yaml"
		app.Path.ConfigFile = filepath.Join(dir, fn)

		err := createConfig(c, app)
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
		app.Path.ConfigFile = filepath.Join(dir, "conf.yaml")

		err := createConfig(c, app)
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
		fn := filepath.Join(dir, "conf.yaml")
		_, err := files.Touch(fn, false)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		app := testutil.SetupApp(t)
		app.Path.ConfigFile = fn
		c := ui.NewConsole()
		err = createConfig(c, app)
		if !errors.Is(err, files.ErrFileExists) {
			t.Fatalf("got error %v, want %v", err, files.ErrFileExists)
		}

		wantErrStr := "file already exists."
		if !strings.Contains(err.Error(), wantErrStr) {
			t.Errorf("error message %q should contain %q", err.Error(), wantErrStr)
		}
	})
}

func TestConfig_PrintJSON(t *testing.T) {
	gitCfg := &application.Git{
		Enabled: true,
		Log:     false,
		GPG:     true,
		Path:    "/some/path",
		Remote:  "git@github.com:ponzipalandri/bookmarks.git",
	}

	fn := filepath.Join(t.TempDir(), application.ConfigFilename)
	app := testutil.SetupApp(t)
	app.Path.ConfigFile = fn
	app.Git = gitCfg

	var buf bytes.Buffer
	c := ui.NewConsole(
		ui.WithWriter(&buf),
		ui.WithTerminal(terminal.New(
			terminal.WithReader(strings.NewReader("y\n")), // input Yes when asking to create file
			terminal.WithWriter(io.Discard),               // send output to null, show no prompt
		)),
	)

	err := createConfig(c, app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = printConfigJSON(c, app)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, gitCfg.Remote) {
		t.Fatalf("strings %q not found", gitCfg.Remote)
	}
	if !strings.Contains(output, gitCfg.Path) {
		t.Fatalf("strings %q not found", gitCfg.Path)
	}
}
