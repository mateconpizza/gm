//nolint:funlen //testing
package appcfg

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/config"
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
		cfg := testutil.SetupConfig(t)
		fn := "config.yaml"
		cfg.Path.ConfigFile = filepath.Join(dir, fn)

		err := createConfig(c, cfg)
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

		cfg := testutil.SetupConfig(t)
		cfg.Flags.Yes = true
		dir := t.TempDir()
		cfg.Path.ConfigFile = filepath.Join(dir, "conf.yaml")

		err := createConfig(c, cfg)
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

		cfg := testutil.SetupConfig(t)
		cfg.Path.ConfigFile = fn
		c := ui.NewConsole()
		err = createConfig(c, cfg)
		if !errors.Is(err, files.ErrFileExists) {
			t.Fatalf("got error %v, want %v", err, files.ErrFileExists)
		}

		wantErrStr := "file already exists."
		if !strings.Contains(err.Error(), wantErrStr) {
			t.Errorf("error message %q should contain %q", err.Error(), wantErrStr)
		}
	})
}

//nolint:paralleltest //using t.Setenv from `testutil.SetupConfig`
func TestConfig_PrintJSON(t *testing.T) {
	gitCfg := &config.Git{
		Enabled: true,
		Log:     false,
		GPG:     true,
		Path:    "/some/path",
		Remote:  "git@github.com:ponzipalandri/bookmarks.git",
	}

	fn := filepath.Join(t.TempDir(), config.ConfigFilename)
	cfg := testutil.SetupConfig(t)
	cfg.Path.ConfigFile = fn
	cfg.Git = gitCfg

	var buf bytes.Buffer
	c := ui.NewConsole(
		ui.WithWriter(&buf),
		ui.WithTerminal(terminal.New(
			terminal.WithReader(strings.NewReader("y\n")), // input Yes when asking to create file
			terminal.WithWriter(io.Discard),               // send output to null, show no prompt
		)),
	)

	err := createConfig(c, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = printConfigJSON(c, cfg)
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
