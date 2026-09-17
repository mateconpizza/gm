package testutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	menu "github.com/mateconpizza/go-fzf"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type MenuRunner struct {
	retcode int
	output  string
}

func NewMenuRunner() *MenuRunner {
	return &MenuRunner{}
}

func (f *MenuRunner) Parse(defaults bool, settings menu.Args) (*menu.RunOptions, error) {
	return &menu.RunOptions{}, nil
}

func (f *MenuRunner) Run(opts *menu.RunOptions) (int, error) {
	opts.Output <- f.output
	return f.retcode, nil
}

func (f *MenuRunner) WithOutput(s string) *MenuRunner {
	f.output = s
	return f
}

func (f *MenuRunner) WithRetCode(i int) *MenuRunner {
	f.retcode = i
	return f
}

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
		Menu: &menucfg.Config{},
	}
}

func NewConsole(t *testing.T, w io.Writer) *ui.Console {
	t.Helper()

	if w == nil {
		w = io.Discard
	}

	tm := terminal.New(
		terminal.WithWriter(w),
	)

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
	tempDir := t.TempDir()

	app := NewApp(t).
		WithHomePath(tempDir)
	_ = app.SetDatabase(app.DBName)

	c := NewConsole(t, io.Discard)

	return deps.New(
		deps.WithApplication(app),
		deps.WithConsole(c),
	)
}

func NewDepsWithRepo(t *testing.T, r *db.SQLite) *deps.Deps {
	t.Helper()
	return NewDeps(t).
		WithRepo(r)
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

	t.Cleanup(func() {
		r.Close()
	})

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

func NewFile(t *testing.T, root, name string, content []byte) error {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), content, 0o600); err != nil {
		t.Fatal(err)
	}
	return nil
}

type FakeResponse struct {
	out string
	err error
}

type FakeTokenResponse struct {
	token string
	resp  FakeResponse
}

type FakeGitExecuter struct {
	calls          [][]string
	out            string // fallback output for unconfigured subcommands
	err            error  // fallback error for unconfigured subcommands
	responses      map[string]FakeResponse
	tokenResponses []FakeTokenResponse
}

func (f *FakeGitExecuter) Output(ctx context.Context, dir string, args ...string) (string, error) {
	cmds := append([]string(nil), args...) // defensive copy - args' backing array can be reused by the caller
	f.calls = append(f.calls, cmds)

	if resp, ok := f.lookup(cmds); ok {
		return resp.out, resp.err
	}
	return f.out, f.err
}

func (f *FakeGitExecuter) Run(ctx context.Context, dir string, w io.Writer, r io.Reader, cmds ...string) error {
	cp := append([]string(nil), cmds...)
	f.calls = append(f.calls, cp)

	resp, ok := f.lookup(cp)
	if !ok {
		resp = FakeResponse{out: f.out, err: f.err}
	}
	if resp.out != "" {
		fmt.Fprint(w, resp.out)
	}
	return resp.err
}

// On configures a canned response for a subcommand.
func (f *FakeGitExecuter) On(subcommand, out string, err error) *FakeGitExecuter {
	if f.responses == nil {
		f.responses = make(map[string]FakeResponse)
	}
	f.responses[subcommand] = FakeResponse{out: out, err: err}
	return f
}

// OnContains configures a response for any call whose args contain token
// anywhere.
func (f *FakeGitExecuter) OnContains(token, out string, err error) *FakeGitExecuter {
	f.tokenResponses = append(f.tokenResponses, FakeTokenResponse{token: token, resp: FakeResponse{out: out, err: err}})
	return f
}

// lookup now checks subcommand-keyed responses first, then falls back to
// token matches in registration order, then f.out/f.err.
func (f *FakeGitExecuter) lookup(cmds []string) (FakeResponse, bool) {
	if len(cmds) >= 2 {
		if resp, ok := f.responses[cmds[1]]; ok {
			return resp, true
		}
	}
	for _, tr := range f.tokenResponses {
		if slices.Contains(cmds, tr.token) {
			return tr.resp, true
		}
	}
	return FakeResponse{}, false
}
