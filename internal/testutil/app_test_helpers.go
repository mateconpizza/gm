package testutil

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

func SetupApp(t *testing.T) *app.Context {
	t.Helper()

	cfg := &config.Config{
		Name:   config.AppName,
		Cmd:    config.AppCommand,
		DBName: config.MainDBName,
		Path:   &config.Path{},
		Flags: &config.Flags{
			ColorStr: "never",
			Color:    false,
		},
		Info: &config.Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: "0.0.1",
		},
		Env: &config.Env{
			Home:   "GOMARKS_HOME",
			Editor: "GOMARKS_EDITOR",
		},
	}

	temp := t.TempDir()
	t.Setenv(cfg.Env.Home, temp)

	cfg.DBPath = filepath.Join(temp, cfg.DBName)
	cfg.Path.Data = temp

	tm := terminal.New(
		terminal.WithContext(t.Context()),
		terminal.WithWriter(io.Discard),
	)

	return app.New(t.Context(),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewConsole(
			ui.WithTerminal(tm),
			ui.WithFrame(frame.New()),
		)),
	)
}
