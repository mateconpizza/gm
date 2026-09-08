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
	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

var ErrInvalidOption = errors.New("invalid option")

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
}

func ReorderDatabase(ctx context.Context, app *application.App, r reorderStore, c *ui.Console) error {
	p := c.Palette()
	y := p.BrightYellow.With(p.Italic).Sprint
	c.NewBannerBuilder().
		WithTitle("Reorder records IDs").
		WithTitleColor(p.BrightRed.With(p.Bold)).
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Warning(y("This operation deletes and recreates all bookmark records to assign new\n")).
		Warning(y("sequential IDs.\n")).
		Rowln().
		Flush()

	if !c.Confirm(ctx, "continue?", "n") {
		return sys.ErrExitFailure
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

	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}

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
	if app.Flags.Yes || app.Flags.Force {
		fmt.Fprintln(d.Writer(), c.SuccessMesg("database dropped"))

		return r.DropSecure(ctx)
	}

	s, err := RepoInfo(ctx, d)
	if err != nil {
		return err
	}

	c.NewBannerBuilder().
		WithTitle("Drop All Records").
		WithTitleColor(c.Palette().BrightRed.With(c.Palette().Bold)).
		WithSubtitle("this action cannot be undone").
		WithComment(" (ctrl-c to exit)").
		Build().
		Rowln().
		Text(s).
		Rowln().
		Flush()

	q := "continue?"
	if r.Name() == application.MainDBName {
		q = c.WarningMesg("dropping \"main\" database, continue?")
	}

	if err := c.ConfirmErr(ctx, q, "n"); err != nil {
		return err
	}

	if err := r.DropSecure(ctx); err != nil {
		return err
	}

	return c.Print(ctx, c.SuccessMesg("database dropped\n"))
}

// Remove removes a repo.
func Remove(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if !files.Exists(app.Path.DB()) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, app.Path.DB())
	}

	c, p := d.Console(), d.Console().Palette()
	if filepath.Base(app.Path.DB()) == application.MainDBName && !app.Flags.Force {
		f := p.BrightYellow.With(p.Italic).Sprint("--force")
		return fmt.Errorf("%w: removing the main database requires %s", ErrInvalidOption, f)
	}

	if !app.Flags.Force && !app.Flags.Yes {
		c.NewBannerBuilder().
			WithTitle("Remove Database/s").WithTitleColor(p.BrightRed.With(p.Bold)).
			WithSubtitle("this action cannot be undone").
			Render()

		fmt.Fprint(d.Writer(), SummaryRepoFromPath(ctx, c, app.Path.DB(), app.Path.Backup()))
		err := c.ConfirmErr(ctx, p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(app.Path.DB())+"?", "n")
		if err != nil {
			return err
		}
	}

	if err := RemoveBackups(ctx, d); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(app.Path.DB()); err != nil {
		return err
	}

	dbName := files.StripExts(filepath.Base(app.Path.DB()))
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	fs, err := Backups(ctx, d)
	if err != nil {
		return err
	}

	if app.Flags.Yes || app.Flags.Force {
		return removeSlicePath(ctx, d, fs)
	}

	p := d.Console().Palette()
	d.Console().NewBannerBuilder().
		WithTitle("Remove backups").WithTitleColor(p.BrightRed.With(p.Bold)).
		WithComment(" (ctrl-c to exit)").
		WithSubtitle("this action cannot be undone").
		Build().
		Rowln().
		Flush()

	filesToRemove, err := selectBackupsInteractive(ctx, d, fs)
	if err != nil {
		return err
	}

	return removeSlicePath(ctx, d, filesToRemove)
}

// Lock locks the database.
func Lock(ctx context.Context, c consolePass, items []string) error {
	for i := range items {
		toLock := items[i]

		if err := locker.IsLocked(toLock); err != nil {
			return err
		}

		if !files.Exists(toLock) {
			return fmt.Errorf("%w: %q", os.ErrNotExist, filepath.Base(toLock))
		}

		if !c.Confirm(ctx, fmt.Sprintf("Lock %q?", filepath.Base(toLock)), "n") {
			continue
		}

		pass, err := c.InputPasswordConfirm(ctx)
		if err != nil {
			return err
		}

		if err := locker.Lock(toLock, pass); err != nil {
			return err
		}

		fmt.Fprintln(c.Writer(), c.SuccessMesg(fmt.Sprintf("database locked: %q", filepath.Base(toLock))))
	}

	return nil
}

// LockDatabase select and lock a database.
func LockDatabase(ctx context.Context, app *application.App) error {
	found := false
	formatter := func(s string) string {
		if !found {
			name, rest, _ := strings.Cut(s, " ")
			if name == app.DBBaseName() {
				return ansi.BrightYellow.Sprint(name) + " " + rest
			}
		}
		return s
	}

	selected, err := NewDatabaseSelector(app).
		WithItemDecorator(formatter).
		WithOpts(
			menu.WithHeader("select a database to lock"),
			menu.WithMultiSelection(),
		).
		Select(ctx)
	if err != nil {
		return err
	}

	return Lock(ctx, ui.DefaultConsole, selected)
}

// UnlockDatabase select and unlock a database.
func UnlockDatabase(ctx context.Context, app *application.App, c *ui.Console) error {
	selected, err := NewDatabaseEncryptedSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeaderKeymaps(),
			menu.WithHeader("select a encrypted database/s to unlock"),
		).
		Select(ctx)
	if err != nil {
		return err
	}

	return Unlock(ctx, c, selected)
}

// UnlockBackup select and unlock a backup.
func UnlockBackup(ctx context.Context, app *application.App, c *ui.Console) error {
	if !files.Exists(app.Path.Backup()) {
		return db.ErrBackupNotFound
	}

	selected, err := NewBackupEncryptedSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeaderKeymaps(),
			menu.WithHeader("select a database/s to unlock"),
		).
		Select(ctx)
	if err != nil {
		return err
	}

	return Unlock(ctx, c, selected)
}

// LockBackup select and lock a backup.
func LockBackup(ctx context.Context, app *application.App, c *ui.Console) error {
	selected, err := NewBackupSelector(app).
		WithOpts(
			menu.WithMultiSelection(),
			menu.WithKeybinds(menu.KeymapToggleAll()),
			menu.WithHeader("select backup/s to lock"),
		).
		Select(ctx)
	if err != nil {
		return err
	}

	f, p := c.Frame(), c.Palette()
	f.Header(fmt.Sprintf("locking %d backups\n", len(selected))).Row("\n").Flush()

	if err := Lock(ctx, c, selected); err != nil {
		if errors.Is(err, sys.ErrActionAborted) || errors.Is(err, terminal.ErrIncorrectAttempts) {
			f.Warning(p.Gray.With(p.Italic).Sprintf("skipped: %s\n", err.Error())).Flush()
		}

		return err
	}

	return nil
}

// Unlock unlocks the database.
func Unlock(ctx context.Context, c consolePass, items []string) error {
	for i := range items {
		rToUnlock := items[i]
		if err := locker.IsLocked(rToUnlock); err == nil {
			return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, filepath.Base(rToUnlock))
		}

		rToUnlock = locker.Extension.Join(rToUnlock)
		slog.Debug("unlocking database", "name", rToUnlock)

		if !files.Exists(rToUnlock) {
			s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
			return fmt.Errorf("%w: %q", os.ErrNotExist, s)
		}

		if err := c.ConfirmErr(ctx, fmt.Sprintf("Unlock %q?", filepath.Base(rToUnlock)), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}

		s, err := c.InputPassword(ctx, "Password: ")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := locker.Unlock(rToUnlock, s); err != nil {
			fmt.Fprintln(c.Writer())
			return fmt.Errorf("%w", err)
		}

		fmt.Fprintln(c.Writer())
		fmt.Fprintln(c.Writer(), c.SuccessMesg("database unlocked"))
	}

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

	r, err := d.Repository()
	if err != nil {
		return nil, err
	}

	bks, err := files.List(app.Path.Backup(), "*_"+r.BaseName()+".db*") // match YYYYMMDD-HHMMSS_dbname.db
	if err != nil {
		return nil, err
	}

	if len(bks) == 0 {
		return nil, db.ErrBackupNotFound
	}

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
		return p.BrightYellow.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
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
	f.Success(txt.PaddedLineWithPad("schema version", p.BrightGreen.Sprint(schemaVer)+"\n", padding)).
		Success(txt.PaddedLineWithPad("sqlite version", p.BrightMagenta.Sprint(sqlVer)+"\n", padding)).
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
		WithTitle("Repository Backups").WithTitleColor(p.BrightMagenta.With(p.Bold)).
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

func Diagnostic(ctx context.Context, d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()

	f := d.Console().Frame()
	p := d.Console().Palette()
	defer f.Flush()

	title := p.BrightYellow.
		Wrap("Database Doctor", p.Bold)

	f.Headerln(title).
		Rowln().
		Midln("integrity_check").
		Midln("foreign_key_check").
		Midln("missing indexes").
		Midln("orphan tags").
		Midln("invalid status").
		Midln("duplicated checksums")

	return nil
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(ctx context.Context, d *deps.Deps, dbs []string) error {
	c, f := d.Console(), d.Console().Frame()

	n := len(dbs)
	if n == 0 {
		return picker.ErrNoItems
	}

	app, err := d.Application(ctx)
	if err != nil {
		return err
	}
	if n > 1 && !app.Flags.Yes {
		for i := range n {
			f.Midln(repoRecordsFromPath(ctx, d.Console(), dbs[i]))
		}
		f.Flush()

		msg := fmt.Sprintf("%s %d item/s", c.Palette().BrightRed.Sprint("removing"), n)
		if err := c.ConfirmErr(ctx, msg+", continue?", "n"); err != nil {
			return err
		}
	}

	sp := rotato.New(
		rotato.WithMessage("removing database..."),
		rotato.WithMessageColor(rotato.FgYellow),
		rotato.WithWriter(c.Writer()),
	)
	sp.Start(ctx)

	for i := range n {
		if err := files.Remove(dbs[i]); err != nil {
			return err
		}
	}

	sp.Done()

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("%d item/s removed", n)))

	return nil
}
