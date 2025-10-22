package testutil

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func SetupConfig(t *testing.T) *config.Config {
	t.Helper()

	return &config.Config{
		Name:   config.AppName,
		Cmd:    config.AppCommand,
		DBName: config.MainDBName,
		Path:   &config.Path{},
		Flags: &config.Flags{
			ColorStr: "never",
			Color:    false,
		},
		Git: &config.Git{},
		Info: &config.Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: "0.0.1",
		},
		Env: &config.Env{
			Home:   config.EnvHome,
			Editor: config.EnvEditor,
		},
	}
}

func SetupApp(t *testing.T) *app.Context {
	t.Helper()

	cfg := SetupConfig(t)
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

func SetupInitializedEmptyDB(t *testing.T, dbPath string) *db.SQLite {
	t.Helper()

	r, err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}

	if err := r.Init(t.Context()); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return r
}

func SetupInitializedDBWithBookmarks(t *testing.T, dbPath string, n int) *db.SQLite {
	t.Helper()
	r := SetupInitializedEmptyDB(t, dbPath)

	if err := r.InsertMany(t.Context(), sliceBookmark(n)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return r
}

func singleBookmark() *bookmark.Bookmark {
	return &bookmark.Bookmark{
		URL:       "https://www.example.com",
		Title:     "Title",
		Tags:      "test,tag1,go",
		Desc:      "Description",
		CreatedAt: "2023-01-01T12:00:00Z",
		LastVisit: "2023-01-01T12:00:00Z",
		Favorite:  true,
	}
}

func sliceBookmark(n int) []*bookmark.Bookmark {
	bs := make([]*bookmark.Bookmark, 0, n)
	for i := range n {
		b := singleBookmark()
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)
		bs = append(bs, b)
	}

	return bs
}

func ConsoleWithInput(t *testing.T, input string) *ui.Console {
	t.Helper()
	term := terminal.New(terminal.WithContext(t.Context()), terminal.WithReader(strings.NewReader(input)))
	return ui.NewConsole(ui.WithTerminal(term))
}
