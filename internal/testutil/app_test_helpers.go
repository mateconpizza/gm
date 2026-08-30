package testutil

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewApp(t *testing.T) *application.App {
	t.Helper()

	return &application.App{
		Name:   application.Name,
		Cmd:    application.Command,
		DBName: application.MainDBName,
		Path:   &application.Path{},
		Flags: &application.Flags{
			ColorStr: "never",
			Color:    false,
		},
		Git: &application.Git{},
		Info: &application.Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: "0.0.1",
		},
		Env: &application.Env{
			Home:   application.EnvHome,
			Editor: application.EnvEditor,
		},
	}
}

func NewConsole(t *testing.T, w io.Writer) *ui.Console {
	t.Helper()

	if w == nil {
		w = io.Discard
	}
	tm := terminal.New(terminal.WithWriter(w))

	return ui.NewConsole(
		ui.WithTerminal(tm),
		ui.WithFrame(frame.New()),
	)
}

func NewTerminal(t *testing.T, w io.Writer) *terminal.Term {
	t.Helper()
	if w == nil {
		w = io.Discard
	}
	return terminal.New(terminal.WithWriter(w))
}

func NewDeps(t *testing.T) *deps.Deps {
	t.Helper()

	app := NewApp(t)
	temp := t.TempDir()

	app.Path.Database = filepath.Join(temp, app.DBName)
	app.Path.Data = temp

	c := NewConsole(t, io.Discard)

	return deps.New(
		deps.WithApplication(app),
		deps.WithConsole(c),
	)
}

func NewInitializedEmptyDB(t *testing.T, dbPath string) *db.SQLite {
	t.Helper()

	r, err := db.Init(t.Context(), dbPath)
	if err != nil {
		t.Fatalf("failed to init DB: %v", err)
	}

	if err := r.Init(t.Context()); err != nil {
		t.Fatalf("failed to initialize schema: %v", err)
	}

	return r
}

func NewInitializedDBWithBookmarks(t *testing.T, dbPath string, n int) *db.SQLite {
	t.Helper()
	r := NewInitializedEmptyDB(t, dbPath)

	if err := r.InsertMany(t.Context(), NewBookmarkSlice(t, n)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return r
}

func NewBookmark(t *testing.T) *bookmark.Bookmark {
	t.Helper()

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

func NewBookmarkSlice(t *testing.T, n int) []*bookmark.Bookmark {
	t.Helper()

	bs := make([]*bookmark.Bookmark, 0, n)
	for i := range n {
		b := NewBookmark(t)
		b.Title = fmt.Sprintf("Title %d", i)
		b.URL = fmt.Sprintf("https://www.example%d.com", i)
		b.Tags = fmt.Sprintf("test,tag%d,go", i)
		b.Desc = fmt.Sprintf("Description %d", i)

		b.GenChecksum()

		bs = append(bs, b)
	}

	return bs
}

func NewConsoleWithInput(t *testing.T, input string) *ui.Console {
	t.Helper()
	term := terminal.New(terminal.WithReader(strings.NewReader(input)))
	return ui.NewConsole(ui.WithTerminal(term))
}
