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
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

func Select(ctx context.Context, d *deps.Deps, ignoreDBPath string) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	dbs, err := files.ListWithExclude(app.Path.Home(), ".db", ignoreDBPath)
	if err != nil {
		return "", err
	}

	m := picker.New[string](
		app,
		menu.WithHeader("choose a database to import from"),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), "{1}", "db info")),
	)

	m.SetFormatter(func(p string) string {
		return summary.RepoRecordsFromPath(ctx, d.Console(), p)
	})

	s, err := m.Select(dbs)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	if !files.Exists(s[0]) {
		return "", fmt.Errorf("%w: %q", db.ErrDBNotFound, s)
	}

	return s[0], nil
}

// SelectBackup lets the user choose a backup and handles decryption if
// needed.
func SelectBackup(ctx context.Context, d *deps.Deps, bks []string) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	c := d.Console()

	m := setupMenu(
		app,
		func(path string) string {
			return summary.BackupWithFmtDateFromPath(ctx, c, path)
		},
		menu.WithHeader("choose a backup to import from"),
	)
	selected, err := m.Select(bks)
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	backupPath := selected[0]

	// Handle locked backups
	if err := locker.IsLocked(backupPath); err != nil {
		if err := Unlock(ctx, d, backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}

		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

// SelectEncrypted prompts the user to select an encrypted backup file from a
// directory.
func SelectEncrypted(ctx context.Context, d *deps.Deps, root string) (string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	m := picker.New[string](
		app,
		menu.WithHeader("select backup to unlock"),
	)

	m.SetFormatter(func(p string) string {
		return summary.BackupWithFmtDateFromPath(ctx, d.Console(), p)
	})

	f, err := files.FindByExtension(root, "enc")
	if err != nil {
		return "", err
	}

	if len(f) == 0 {
		return "", sys.ErrExitFailure
	}

	f, err = m.Select(f)
	if err != nil {
		return "", err
	}

	return f[0], nil
}

func LoadFromMenu(ctx context.Context, app *application.App) error {
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

func selectBackups(ctx context.Context, d *deps.Deps, header string) ([]string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	fs, err := files.FindByExtension(app.Path.Backup(), "db")
	if err != nil {
		return fs, fmt.Errorf("selectBackups: %w", err)
	}

	p := d.Console().Palette()
	m := setupMenu(
		app,
		func(path string) string {
			s := summary.RepoRecordsFromPath(ctx, d.Console(), path)
			if s == "error" {
				s = path + p.BrightRed.Sprint(" (err on read)")
			}
			return s
		},
		menu.WithHeader(header),
		menu.WithMultiSelection(),
	)

	repos, err := m.Select(fs)
	if err != nil {
		return repos, fmt.Errorf("%w", err)
	}

	return repos, nil
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

	m := setupMenu(
		app,
		func(item string) string {
			return summary.BackupWithFmtDateFromPath(ctx, c, item)
		},
		menu.WithMultiSelection(),
		menu.WithHeader(fmt.Sprintf("select backup/s from %q", app.DBBaseName())),
	)

	return m.Select(fs)
}

func setupMenu[T comparable](app *application.App, formatter menu.FmtFunc[T], opts ...menu.Option) *menu.Menu[T] {
	opts = append(
		opts,
		menu.WithCycle(),
		menu.WithPreviewCmd(picker.PreviewCmd(app.Command(), "./backup/{1}", "db info")),
	)

	m := picker.New[T](app, opts...)
	m.SetFormatter(formatter)
	return m
}

func formatDatabaseFn(ctx context.Context, p *ansi.Palette, path string, pad int) string {
	name := filepath.Base(path)
	r, err := db.New(ctx, path)
	if err != nil {
		return name + ": " + err.Error()
	}
	defer r.Close()

	stats := db.NewStats()
	if err := r.Stats(ctx, stats); err != nil {
		return name + ": " + err.Error()
	}

	createdAt, err := r.Metadata(db.MetaKeyCreatedAt)
	if err != nil {
		createdAt = "err"
	}

	main := p.Gray.Sprintf(
		"(%d bookmarks %d tags)",
		stats.Bookmarks,
		stats.Tags,
	)

	createdAt = summary.RelativeDays(createdAt)
	if createdAt == "" {
		createdAt = "err"
	}

	name = files.StripExts(r.Name())
	return txt.PaddedLineWithPad(
		name,
		main+" "+createdAt,
		pad,
	)
}
