package dbops

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/files"
)

// SelectBackup lets the user choose a backup and handles decryption if
// needed.
func SelectBackup(ctx context.Context, d *deps.Deps, bks []string) (string, error) {
	c := d.Console()
	app, err := d.Application(ctx)
	if err != nil {
		return "", err
	}

	m := setupMenu(
		app,
		func(path *string) string {
			return summary.BackupWithFmtDateFromPath(ctx, c, *path)
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
		if err := UnlockRepo(ctx, d, backupPath); err != nil {
			return "", fmt.Errorf("%w", err)
		}

		backupPath = strings.TrimSuffix(backupPath, ".enc")
	}

	return backupPath, nil
}

func SelectBackupMany(ctx context.Context, d *deps.Deps, root, header string) ([]string, error) {
	fs, err := files.FindByExtList(root, "db")
	if err != nil {
		return fs, fmt.Errorf("%w", err)
	}

	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	p := d.Console().Palette()
	m := setupMenu(
		app,
		func(path *string) string {
			s := summary.RepoRecordsFromPath(ctx, d.Console(), *path)
			if s == "error" {
				s = *path + p.BrightRed.Sprint(" (err on read)")
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
		func(item *string) string {
			return summary.BackupWithFmtDateFromPath(ctx, c, *item)
		},
		menu.WithMultiSelection(),
		menu.WithHeader(fmt.Sprintf("select backup/s from %q", app.DBBaseName())),
	)

	return m.Select(fs)
}

func setupMenu[T comparable](app *application.App, formatter func(item *T) string, opts ...menu.Option) *menu.Menu[T] {
	opts = append(
		opts,
		menu.WithArgs("--cycle"),
		menu.WithPreview(app.PreviewCmd("./backup/{1}", "db info")),
	)

	m := picker.New[T](app, opts...)
	m.SetFormatter(formatter)
	return m
}
