package dbops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	menu "github.com/mateconpizza/go-fzf"
	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

var (
	ErrInvalidOption = errors.New("invalid option")
	ErrInvalidRename = errors.New("invalid rename")
)

type reorderStore interface {
	ReorderIDs(ctx context.Context) error
	Backup(ctx context.Context, destRoot string) (string, error)
}

type consolePass interface {
	Confirm(ctx context.Context, q, def string) bool
	ConfirmErr(ctx context.Context, q, def string) error
	InputPassword(ctx context.Context, s string) (string, error)
	InputPasswordConfirm(ctx context.Context) (string, error)
	SuccessMesg(a ...any) string
	Writer() io.Writer
	Term() *terminal.Term
}

func ReorderDatabase(ctx context.Context, app *application.App, r reorderStore, c *ui.Console) error {
	p := c.Palette()
	y := p.BrightYellow.With(p.Italic).Sprint
	c.NewBannerBuilder().
		WithTitle("Reorder records IDs").
		WithTitleColor(p.BrightRed.With(p.Bold).Sprint).
		WithTitleGlyph(c.Glyphs().Warning).
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Warning(y("This operation deletes and recreates all bookmark records to assign new\n")).
		Warning(y("sequential IDs.\n")).
		Rowln().
		Flush()

	if !c.Confirm(ctx, "continue?", "n") {
		return app.Failure()
	}

	if c.Confirm(ctx, "create backup?", "y") {
		if err := os.MkdirAll(app.Path.Backup(), files.DirPerm); err != nil {
			return err
		}
		newBkPath, err := r.Backup(ctx, app.Path.Backup())
		if err != nil {
			return err
		}
		_ = c.Term().Print(ctx, c.Success(fmt.Sprintf("backup created: %q\n", filepath.Base(newBkPath))).String())
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := r.ReorderIDs(ctx); err != nil {
		return err
	}
	return c.Term().Print(ctx, c.SuccessMesg("renumber bookmark IDs sequentially.\n"))
}

func VacuumDatabase(ctx context.Context, app *application.App) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	r, err := db.Open(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	return r.Vacuum(ctx)
}

func SetDefault(ctx context.Context, app *application.App, filename string) error {
	filename = files.StripExts(filename)
	if filename == "" {
		return fmt.Errorf("%w: %q", ErrInvalidOption, filename)
	}

	if filename == "default" {
		filename = application.MainDBName
	}

	if err := app.SetDatabase(filename); err != nil {
		return err
	}

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	return app.WriteConfig(true)
}

// Drop drops a database.
func Drop(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	c := d.Console()
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := slog.With("op", "drop", "repo", r.Name())
	log.Debug("starting", "yes", app.Flags.Yes, "force", app.Flags.Force)

	if app.Flags.Yes || app.Flags.Force {
		log.Debug("confirmation skipped by flag")
		if err := r.DropSecure(ctx); err != nil {
			log.Error("drop failed", "err", err)
			return fmt.Errorf("drop %q: %w", r.Name(), err)
		}
		log.Info("dropped")

		fmt.Fprintln(d.Writer(), c.SuccessMesg("database dropped"))

		return nil
	}

	s, err := RepoInfo(ctx, d)
	if err != nil {
		return err
	}

	c.NewBannerBuilder().
		WithTitle("Drop All Records").
		WithTitleColor(c.Palette().BrightRed.With(c.Palette().Bold).Sprint).
		WithTitleGlyph(c.Glyphs().Warning).
		WithSubtitle("this action cannot be undone").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln().
		Text(s).
		Rowln().
		Flush()

	q := "continue?"
	if r.Name() == application.MainDBName {
		log.Warn("dropping main database")
		q = c.WarningMesg("dropping \"main\" database, continue?")
	}

	if err := c.ConfirmErr(ctx, q, "n"); err != nil {
		log.Debug("aborted by user", "err", err)
		return err
	}

	if err := r.DropSecure(ctx); err != nil {
		log.Error("drop failed", "err", err)
		return fmt.Errorf("drop %q: %w", r.Name(), err)
	}
	log.Info("dropped")

	return c.Print(ctx, c.SuccessMesg("database dropped\n"))
}

// RemoveBackups removes backups.
func RemoveBackups(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("op", "remove-backups")

	fs, err := Backups(ctx, d)
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		log.Debug("no database backups found")
		return db.ErrBackupNotFound
	}

	log.Debug("database backups found", "count", len(fs))

	if app.Flags.Yes || app.Flags.Force {
		log.Debug("removing backups without interactive selection")
		return removeSlicePath(ctx, d, fs)
	}

	c, p := d.Console(), d.Console().Palette()

	log.Debug("requesting backup selection")

	c.NewBannerBuilder().
		WithTitle("Remove backups").
		WithTitleColor(p.BrightRed.With(p.Bold).Sprint).
		WithTitleGlyph(app.Glyphs().Del).
		WithComment(" (ctrl-c to exit)").
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Flush()

	filesToRemove, err := selectBackupsInteractive(ctx, d, fs)
	if err != nil {
		log.Debug("backup selection failed", "error", err)
		return err
	}

	log.Debug("backups selected", "count", len(filesToRemove))

	return removeSlicePath(ctx, d, filesToRemove)
}

// Lock locks the database.
func Lock(ctx context.Context, c consolePass, items []string, logger *slog.Logger) error {
	log := logger.With("fn", "lock", "count", len(items))

	var locked, skipped int

	c.Term().SetLogger(log)

	for _, item := range items {
		name := filepath.Base(item)
		itemLog := log.With("database", name)

		if err := ctx.Err(); err != nil {
			itemLog.Debug("database locking canceled", "error", err)
			return err
		}

		if err := locker.IsLocked(item); err != nil {
			itemLog.Error("failed to check database lock state", "error", err)
			return err
		}

		if !files.Exists(item) {
			itemLog.Warn("database file not found")
			return fmt.Errorf("%w: %q", os.ErrNotExist, name)
		}

		if !c.Confirm(ctx, fmt.Sprintf("Lock %q?", name), "n") {
			skipped++
			itemLog.Debug("database locking skipped")
			continue
		}

		pass, err := c.InputPasswordConfirm(ctx)
		if err != nil {
			itemLog.Debug("password confirmation failed", "error", err)
			return err
		}

		itemLog.Debug("locking database")

		if err := locker.Lock(ctx, item, pass, itemLog); err != nil {
			itemLog.Error("failed to lock database", "error", err)
			return err
		}

		fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("database locked: %q", name)))

		locked++
		itemLog.Debug("database locked successfully")
	}

	log.Debug("database locking completed",
		"locked", locked,
		"skipped", skipped,
	)

	return nil
}

// LockDatabase select and lock a database.
func LockDatabase(ctx context.Context, app *application.App, c *ui.Console, args []string) error {
	log := app.Logger().With("op", "lock-database")

	found := false
	formatter := func(s string) string {
		if !found {
			name, rest, _ := strings.Cut(s, " ")
			if name == app.DBBaseName() {
				found = true
				return ansi.BrightYellow.Sprint(name) + " " + rest
			}
		}
		return s
	}

	log.Debug("selecting databases to lock")

	selected, err := NewDatabaseSelector(app).
		WithItemDecorator(formatter).
		WithOpts(
			menu.WithHeader("select a database to lock"),
			menu.WithMultiSelection(),
			menu.WithArgs(func(b *menu.ArgsBuilder) *menu.ArgsBuilder {
				if len(args) > 0 {
					return b.Custom("--query=" + args[0])
				}
				return b
			}),
		).
		WithLogger(log.With("selector", "database")).
		Select(ctx)
	if err != nil {
		log.Debug("database selection failed", "error", err)
		return err
	}

	log.Debug("databases selected", "count", len(selected))

	return Lock(ctx, ui.NewDefaultConsole(app, app.Exit), selected, log)
}

// UnlockDatabase select and unlock a database.
func UnlockDatabase(ctx context.Context, app *application.App, c *ui.Console, args []string) error {
	log := app.Logger().With("op", "unlock-database")

	log.Debug("selecting encrypted databases")

	selected, err := NewDatabaseEncryptedSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeaderKeymaps(),
			menu.WithHeader("select a encrypted database/s to unlock"),
			menu.WithArgs(func(b *menu.ArgsBuilder) *menu.ArgsBuilder {
				if len(args) > 0 {
					return b.Custom("--query=" + args[0])
				}
				return b
			}),
		).
		WithLogger(log.With("selector", "enc-database")).
		Select(ctx)
	if err != nil {
		log.Debug("database selection failed", "error", err)
		return err
	}

	log.Debug("encrypted databases selected", "count", len(selected))

	return Unlock(ctx, c, selected, log)
}

// UnlockBackup select and unlock a backup.
func UnlockBackup(ctx context.Context, app *application.App, c *ui.Console) error {
	log := app.Logger().With("op", "unlock-backup")

	if !files.Exists(app.Path.Backup()) {
		log.Error("root backup folder not found")
		return db.ErrBackupNotFound
	}

	log.Debug("selecting encrypted backup")

	selected, err := NewBackupEncryptedSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeaderKeymaps(),
			menu.WithHeader("select a database/s to unlock"),
		).
		Select(ctx)
	if err != nil {
		log.Debug("database selection failed", "error", err)
		return err
	}

	log.Debug("encrypted backup selected", "count", len(selected))

	return Unlock(ctx, c, selected, log)
}

// LockBackup select and lock a backup.
func LockBackup(ctx context.Context, app *application.App, c *ui.Console) error {
	log := app.Logger().With("op", "lock-backup")

	log.Debug("selecting backups to lock")

	selected, err := NewBackupSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeader("select backup/s to lock"),
		).
		WithLogger(log.With("selector", "backup")).
		Select(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Debug("backup selection canceled")
		} else {
			log.Error("failed to select backups", "error", err)
		}
		return err
	}

	log.Debug("backups selected", "count", len(selected))

	f, p := c.Frame(), c.Palette()
	f.Header(fmt.Sprintf("locking %d backups\n", len(selected))).Row("\n").Flush()

	log.Debug("locking selected backups", "count", len(selected))

	if err := Lock(ctx, c, selected, log); err != nil {
		if errors.Is(err, app.Abort()) || errors.Is(err, terminal.ErrIncorrectAttempts) {
			log.Debug("backup locking skipped", "error", err)
			f.Warning(p.Gray.With(p.Italic).Sprintf("skipped: %s\n", err.Error())).Flush()
		} else {
			log.Error("failed to lock backups", "error", err)
		}

		return err
	}

	log.Debug("backup locking completed", "count", len(selected))

	return nil
}

// Unlock unlocks the database.
func Unlock(ctx context.Context, c consolePass, items []string, logger *slog.Logger) error {
	log := logger.With("fn", "unlock")

	log.Debug("unlocking databases", "count", len(items))

	unlocked := 0

	c.Term().SetLogger(log)

	for _, item := range items {
		rToUnlock := item
		name := filepath.Base(rToUnlock)

		if err := locker.IsLocked(rToUnlock); err == nil {
			log.Debug("database is already unlocked", "name", name)
			return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, name)
		}

		rToUnlock = locker.Extension.Join(rToUnlock)
		name = filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))

		log.Debug("checking encrypted database", "name", name)

		if !files.Exists(rToUnlock) {
			log.Warn("encrypted database not found", "name", name)
			return fmt.Errorf("%w: %q", os.ErrNotExist, name)
		}

		if err := c.ConfirmErr(ctx, fmt.Sprintf("Unlock %q?", filepath.Base(rToUnlock)), "y"); err != nil {
			log.Debug("database unlock canceled", "name", name, "error", err)
			return err
		}

		password, err := c.InputPassword(ctx, "Password: ")
		if err != nil {
			log.Debug("password input failed", "name", name, "error", err)
			return err
		}

		log.Debug("decrypting database", "name", name)

		if err := locker.Unlock(ctx, rToUnlock, password, log); err != nil {
			log.Error("failed to unlock database", "name", name, "error", err)
			fmt.Fprintln(c.Writer())
			return err
		}

		fmt.Fprintln(c.Writer())
		fmt.Fprintln(c.Writer(), c.SuccessMesg("database unlocked"))

		unlocked++
		log.Debug("database unlocked", "name", name)
	}

	log.Debug("database unlock completed", "count", unlocked)

	return nil
}

func NewBackup(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	srcPath := app.Path.DB()
	if !files.Exists(srcPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, srcPath)
	}

	if files.IsEmpty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	s, err := RepoInfo(ctx, d)
	if err != nil {
		return err
	}
	fmt.Fprint(d.Writer(), s)

	c := d.Console()
	f, p := c.Frame(), c.Palette()
	f.Reset().Row("\n").Flush()

	if !app.Flags.Yes {
		if err := c.ConfirmErr(ctx, "create "+p.BrightGreen.Wrap("backup", p.Italic), "y"); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(app.Path.Backup(), files.DirPerm); err != nil {
		return err
	}

	r, err := d.Repository()
	if err != nil {
		return err
	}

	newBkPath, err := r.Backup(ctx, app.Path.Backup())
	if err != nil {
		return err
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("backup created: %q", filepath.Base(newBkPath))))

	return nil
}

func Backups(ctx context.Context, d *deps.Deps) ([]string, error) {
	app, err := d.Application(ctx)
	if err != nil {
		return nil, err
	}

	log := app.Logger().With("op", "list-backups")

	r, err := d.Repository()
	if err != nil {
		log.Error("failed to get repository", "error", err)
		return nil, err
	}

	pattern := "*_" + r.BaseName() + ".db*"

	bks, err := files.List(app.Path.Backup(), pattern)
	if err != nil {
		log.Error("failed to list database backups", "error", err)
		return nil, err
	}

	log.Debug("database backups listed", "count", len(bks))

	return bks, nil
}

// MigrationsStatus prints the current database schema and SQLite versions.
func MigrationsStatus(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	c := d.Console()
	p, f := c.Palette(), c.Frame()
	header := func() string {
		return p.BrightYellow.Wrap(c.Glyphs().Square+" ", p.Bold)
	}
	f.CustomFunc(header, p.Bold.Sprint("Configuring database")).Ln()

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	if err = r.UpdateAppVersion(ctx, app.Version()); err != nil {
		return fmt.Errorf("app version update failed: %w", err)
	}

	schemaVer, err := r.CurrentSchemaVersion(ctx)
	if err != nil {
		return err
	}

	sqlVer, err := r.SQLiteVersion(ctx)
	if err != nil {
		return err
	}

	const padding = 28
	f.Success(txt.PaddedLineWithWidth("schema version", p.BrightGreen.Sprint(schemaVer)+"\n", padding)).
		Success(txt.PaddedLineWithWidth("sqlite version", p.BrightMagenta.Sprint(sqlVer)+"\n", padding)).
		Rowln().
		Flush()

	return nil
}

func BackupList(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}

	stats := db.NewStats()
	if err := r.Stats(ctx, stats); err != nil {
		return err
	}

	p := d.Console().Palette()
	info := p.Dim.With(p.Italic).Sprintf(" (%d bookmarks)", stats.Bookmarks)
	name := p.BrightYellow.With(p.Bold).Sprint(files.StripExts(r.Name()))
	repo := p.Dim.With(p.Italic).Sprint("repo: " + name)
	d.Console().NewBannerBuilder().
		WithTitle("Repository Backups").
		WithTitleColor(p.BrightMagenta.With(p.Bold).Sprint).
		WithSubtitle("latest backup snapshots").
		Build().
		Rowln().
		Midln(repo + info).
		Rowln().
		Flush()

	bkDetail, err := repoBackupListDetail(ctx, d, true)
	if err != nil {
		return err
	}

	fmt.Fprint(d.Writer(), bkDetail)

	return nil
}

// IsInitializedFromPath checks if the database is initialized.
func IsInitializedFromPath(ctx context.Context, p string) (bool, error) {
	log := slog.Default().With("fn", "db-is-initialized", "path", p)

	r, err := db.New(ctx, p)
	if err != nil {
		return false, err
	}
	defer r.Close()

	ok := r.IsInitialized(ctx)
	log.Debug("initialization checked", "initialized", ok)

	return ok, nil
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(ctx context.Context, d *deps.Deps, dbs []string) error {
	c, f := d.Console(), d.Console().Frame()

	n := len(dbs)

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	log := app.Logger().With("op", "remove-backup-files")

	if n == 0 {
		log.Debug("no backup files selected")
		return picker.ErrNoItems
	}

	log.Debug("removing backup files", "count", n)

	if n > 1 && !app.Flags.Yes && !app.Flags.Force {
		for i := range n {
			f.Midln(repoRecordsFromPath(ctx, c, dbs[i]))
		}
		f.Flush()

		msg := fmt.Sprintf("%s %d item/s", c.Palette().BrightRed.Sprint("removing"), n)
		if err := c.ConfirmErr(ctx, msg+", continue?", "n"); err != nil {
			log.Debug("backup removal canceled", "error", err)
			return err
		}
	}

	removed := 0

	for _, path := range dbs {
		if err := files.Remove(path); err != nil {
			log.Error("failed to remove backup",
				"name", filepath.Base(path),
				"removed", removed,
				"total", n,
				"error", err,
			)
			return err
		}

		removed++
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("%d item/s removed", removed)))

	log.Debug("backup files removed", "count", removed)

	return nil
}
