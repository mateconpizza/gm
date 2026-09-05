package dbops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
)

var ErrNoItems = errors.New("no items")

type FmtFunc func(string) string

// DatabaseSelector encapsulates options for listing and picking databases or backups.
type DatabaseSelector struct {
	app        *application.App
	root       string
	ext        string
	exclutions []string
	filter     func(string) bool
	preview    string
	itemFmt    func(ctx context.Context, p *ansi.Palette, path string, maxWidth int) string
	fmtFunc    FmtFunc
}

// NewDatabaseSelector creates a default selector for main databases.
func NewDatabaseSelector(app *application.App) *DatabaseSelector {
	return &DatabaseSelector{
		app:     app,
		root:    app.Path.Home(),
		ext:     "db",
		preview: app.Command() + " db info --db {1} --color=always",
		itemFmt: formatDatabaseFn,
	}
}

// NewBackupSelector creates a selector specifically for backup files.
func NewBackupSelector(app *application.App) *DatabaseSelector {
	return &DatabaseSelector{
		app:     app,
		root:    app.Path.Backup(),
		ext:     "db",
		preview: app.Command() + " --color=always --db=./backup/{1} db info",
		itemFmt: formatBackupFn,
	}
}

// NewDatabaseEncryptedSelector creates a selector specifically for encrypted
// backup files.
func NewDatabaseEncryptedSelector(app *application.App) *DatabaseSelector {
	return &DatabaseSelector{
		app:     app,
		root:    app.Path.Home(),
		ext:     locker.Extension,
		itemFmt: formatBackupFn,
	}
}

// NewBackupEncryptedSelector creates a default selector for encrypted
// databases.
func NewBackupEncryptedSelector(app *application.App) *DatabaseSelector {
	return &DatabaseSelector{
		app:     app,
		root:    app.Path.Backup(),
		ext:     locker.Extension,
		itemFmt: formatBackupFn,
	}
}

// WithCustomFormatter allows overriding the external modifier format function.
func (s *DatabaseSelector) WithCustomFormatter(fn FmtFunc) *DatabaseSelector {
	s.fmtFunc = fn
	return s
}

func (s *DatabaseSelector) WithExclutions(exc ...string) *DatabaseSelector {
	s.exclutions = append(s.exclutions, exc...)
	return s
}

func (s *DatabaseSelector) WithFilter(fn func(string) bool) *DatabaseSelector {
	s.filter = fn
	return s
}

// Select runs the interactive picker dialog.
func (s *DatabaseSelector) Select(ctx context.Context, opts ...menu.Option) ([]string, error) {
	dbs, err := files.ListWithExclude(s.root, s.ext, s.exclutions...)
	if err != nil {
		return nil, err
	}

	if s.filter != nil {
		filtered := make([]string, 0, len(dbs))
		for i := range dbs {
			if s.filter(dbs[i]) {
				filtered = append(filtered, dbs[i])
			}
		}

		dbs = filtered
	}

	if len(dbs) == 0 {
		return nil, ErrNoItems
	}

	var maxWidth int
	for _, path := range dbs {
		name := files.StripExts(filepath.Base(path))
		maxWidth = max(maxWidth, utf8.RuneCountInString(name))
	}

	p := ansi.NewPalette()
	if s.itemFmt == nil {
		s.itemFmt = defaultFmt
	}

	formatItem := func(path string) string {
		formatted := s.itemFmt(ctx, p, path, maxWidth)
		if s.fmtFunc != nil {
			return s.fmtFunc(formatted)
		}
		return formatted
	}

	opts = append(opts,
		menu.WithDefaults(s.app.Menu.Defaults),
		menu.WithAnsi(),
		menu.WithOutputColor(s.app.Flags.Color),
		menu.WithHeaderKeymaps(),
		menu.WithPreviewWindow("right,45%"),
		menu.WithPreviewCmd(s.preview),
	)

	m := picker.New[string](s.app, opts...)
	m.SetFormatter(formatItem)

	selected, err := m.Select(dbs)
	if errors.Is(err, menu.ErrActionAborted) {
		return nil, sys.ErrActionAborted
	}

	return selected, nil
}

func defaultFmt(ctx context.Context, p *ansi.Palette, path string, maxWidth int) string {
	return path
}

func LoadFromMenu(ctx context.Context, app *application.App) error {
	load := menu.NewKeymap().
		WithBind(menu.KeyEnter).
		WithDesc("load").
		WithBecome(app.Command() + " --menu --db={1} --output=" + app.Menu.Format)

	setDefault := menu.NewKeymap().
		WithBind(menu.KeyCtrlS).
		WithDesc("set-as-default").
		WithExecute(app.Command() + " db use {1}")

	_, err := NewDatabaseSelector(app).
		Select(ctx, menu.WithKeybinds(load, setDefault))
	if err != nil {
		return err
	}

	return nil
}

func LoadFromMenuOld(ctx context.Context, app *application.App) error {
	dbs, err := files.ListWithExclude(app.Path.Data, "db")
	if err != nil {
		return err
	}

	var maxWidth int
	for _, path := range dbs {
		name := files.StripExts(filepath.Base(path))
		maxWidth = max(maxWidth, utf8.RuneCountInString(name))
	}

	load := menu.NewKeymap().
		WithBind(menu.KeyEnter).
		WithDesc("load").
		WithBecome(app.Command() + " --menu --db={1} --output=" + app.Menu.Format)

	setDefault := menu.NewKeymap().
		WithBind(menu.KeyCtrlS).
		WithDesc("set-as-default").
		WithExecute(app.Command() + " db use {1}")

	p := ansi.NewPalette()
	found := false
	formatDB := func(path string) string {
		formatted := formatDatabaseFn(ctx, p, path, maxWidth)
		if found || filepath.Base(path) != app.DBName {
			return formatted
		}

		found = true
		return p.BrightYellow.Sprint(formatted)
	}

	m := picker.New[string](
		app,
		menu.WithDefaults(app.Menu.Defaults),
		menu.WithAnsi(),
		menu.WithOutputColor(app.Flags.Color),
		menu.WithKeybinds(load, setDefault),
		menu.WithHeader("select a database"),
		menu.WithHeaderKeymaps(),
		menu.WithPreviewWindow("right,45%"),
		menu.WithPreviewCmd(app.Command()+" db info --db {1} --color=always"),
	)
	m.SetFormatter(formatDB)

	_, err = m.Select(dbs)
	if errors.Is(err, menu.ErrActionAborted) {
		return nil
	}

	return err
}

// selectBackupsInteractive prompts user for backup selection.
func selectBackupsInteractive(ctx context.Context, d *deps.Deps, fs []string) ([]string, error) {
	c, p := d.Console(), d.Console().Palette()

	for {
		opt, err := c.Choose(ctx, p.BrightRed.Wrap("remove", p.Bold)+" backups?", []string{"all", "no", "select"}, "n")
		if err != nil {
			return nil, err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			c.ReplaceLine(c.Warning(p.BrightYellow.Sprint("skipping") + " backup/s").StringReset())
			return nil, nil
		case "a", "all":
			return fs, nil
		case "s", "select":
			return selectBackupsToRemove(ctx, d, fs)
		}
	}
}

// selectBackupsToRemove displays interactive menu for backup selection.
func selectBackupsToRemove(ctx context.Context, d *deps.Deps, fs []string) ([]string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	if app.Flags.Yes || app.Flags.Force {
		return fs, nil
	}

	c := d.Console()
	c.SetReader(os.Stdin)
	c.SetWriter(os.Stdout)

	all := app.Flags.All
	filter := func(s string) bool {
		if all {
			return true
		}
		base := filepath.Base(s)
		_, name, _ := strings.Cut(base, "_")
		return name == filepath.Base(app.Path.DB())
	}

	header := func() string {
		if all {
			return "all"
		}
		return app.DBBaseName()
	}

	return NewBackupSelector(app).
		WithFilter(filter).
		Select(ctx,
			menu.WithMultiSelection(),
			menu.WithHeader(fmt.Sprintf(
				"select backup/s from %q %s %s",
				header(),
				txt.GlyphBulletPoint,
				ansi.BrightRed.Wrap("this action cannot be undone", ansi.Bold),
			)),
		)
}
