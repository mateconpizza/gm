package dbops

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrInvalidOption = errors.New("invalid option")

func ReorderDatabase(ctx context.Context, app *application.App) error {
	r, err := db.New(ctx, app.Path.DB())
	if err != nil {
		return err
	}
	defer r.Close()

	c := ui.NewDefaultConsole(ctx, nil)
	f, p := c.Frame(), c.Palette()

	header := func() string {
		return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}
	title := p.BrightRed.
		Wrap("Reorder records IDs", p.Bold)
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	warn := `This operation deletes and recreates all bookmark records to assign new
sequential IDs.`

	f.CustomFunc(header, title).Ln().
		Headerln(subtitle).
		Rowln()

	w := strings.SplitSeq(warn, "\n")
	for s := range w {
		if s == "" {
			f.Rowln()
			continue
		}
		f.Warning(p.BrightYellow.With(p.Italic).Sprint(s)).Ln()
	}

	f.Rowln().Flush()

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
		_ = c.Print(ctx, c.Success(fmt.Sprintf("backup created: %q\n", filepath.Base(newBkPath))).String())
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := r.ReorderIDs(ctx); err != nil {
		return err
	}
	return c.Print(ctx, c.SuccessMesg("renumber bookmark IDs sequentially.\n"))
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
	filename = files.StripSuffixes(filename)
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

	f, p := c.Frame(), c.Palette()
	title := p.BrightRed.
		Wrap("Drop All Records", p.Bold)
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	header := func() string {
		return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
	}

	s, err := summary.Info(ctx, d)
	if err != nil {
		return err
	}

	f.CustomFunc(header, title+comment).Ln().
		Headerln(subtitle).
		Rowln().
		Text(s).
		Rowln().Flush()

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

	if err := c.Term().Print(ctx, c.SuccessMesg("database dropped\n")); err != nil {
		return err
	}

	return gitops.Drop(ctx, app, c)
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
		title := p.BrightRed.With(p.Bold).Sprint("Remove Database/s")
		subtitle := p.Dim.With(p.Italic).Sprint("this action cannot be undone")
		header := func() string {
			return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold)
		}

		c.Frame().
			CustomFunc(header, title).Ln().
			Headerln(subtitle).
			Rowln().
			Flush()

		fmt.Fprint(d.Writer(), summary.RepoFromPath(ctx, d, app.Path.DB(), app.Path.Backup()))
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

	dbName := files.StripSuffixes(filepath.Base(app.Path.DB()))
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(ctx context.Context, d *deps.Deps) error {
	app, err := d.Application(ctx)
	if err != nil {
		return err
	}

	fs, err := files.List(app.Path.Backup(), "*_"+app.DBBaseName()+".db*") // match YYYYMMDD-HHMMSS_dbname.db
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}
	if app.Flags.Yes || app.Flags.Force {
		return removeSlicePath(ctx, d, fs)
	}

	p := d.Console().Palette()
	header := func() string { return p.BrightRed.Wrap(txt.GlyphSmallSquare.Prefix(" "), p.Bold) }
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")

	f := d.Console().Frame()
	f.CustomFunc(header, p.BrightRed.Sprint("Remove")+" backups"+comment).Ln().
		Headerln(subtitle).
		Rowln().
		Flush()

	filesToRemove, err := selectBackupsInteractive(ctx, d, fs)
	if err != nil {
		return err
	}

	return removeSlicePath(ctx, d, filesToRemove)
}

// Lock locks the database.
func Lock(ctx context.Context, d *deps.Deps, rToLock string) error {
	c := d.Console()
	if err := locker.IsLocked(rToLock); err != nil {
		return fmt.Errorf("%w", err)
	}

	if !files.Exists(rToLock) {
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, filepath.Base(rToLock))
	}

	if err := c.ConfirmErr(ctx, fmt.Sprintf("Lock %q?", filepath.Base(rToLock)), "n"); err != nil {
		if errors.Is(err, sys.ErrActionAborted) {
			return nil
		}
		return err
	}

	pass, err := passwordConfirm(ctx, c)
	if err != nil {
		return err
	}

	if err := locker.Lock(rToLock, pass); err != nil {
		return err
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("database locked: %q", filepath.Base(rToLock))))

	return nil
}

func LockBackup(ctx context.Context, d *deps.Deps) error {
	fs, err := selectBackups(ctx, d, "select backup/s to lock")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	c := d.Console()
	f, p := c.Frame(), c.Palette()
	f.Header(fmt.Sprintf("locking %d backups\n", len(fs))).Row("\n").Flush()

	for _, r := range fs {
		if err := Lock(ctx, d, r); err != nil {
			if errors.Is(err, sys.ErrActionAborted) || errors.Is(err, terminal.ErrIncorrectAttempts) {
				f.Warning(p.Gray.With(p.Italic).Sprintf("skipped: %s\n", err.Error())).Flush()
				continue
			}

			return err
		}
	}

	return nil
}

// Unlock unlocks the database.
func Unlock(ctx context.Context, d *deps.Deps, rToUnlock string) error {
	if err := locker.IsLocked(rToUnlock); err == nil {
		return fmt.Errorf("%w: %q", locker.ErrFileUnlocked, filepath.Base(rToUnlock))
	}

	rToUnlock = files.EnsureSuffix(rToUnlock, locker.Extension)
	slog.Debug("unlocking database", "name", rToUnlock)

	if !files.Exists(rToUnlock) {
		s := filepath.Base(strings.TrimSuffix(rToUnlock, ".enc"))
		return fmt.Errorf("%w: %q", files.ErrFileNotFound, s)
	}

	c := d.Console()
	if err := c.ConfirmErr(ctx, fmt.Sprintf("Unlock %q?", filepath.Base(rToUnlock)), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}

	s, err := c.InputPassword(ctx, "Password: ")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := locker.Unlock(rToUnlock, s); err != nil {
		fmt.Fprintln(d.Writer())
		return fmt.Errorf("%w", err)
	}

	fmt.Fprintln(d.Writer())
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database unlocked"))

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

	if files.Empty(srcPath) {
		return fmt.Errorf("%w", db.ErrDBEmpty)
	}
	s, err := summary.Info(ctx, d)
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

	if app.Flags.Force {
		slog.Debug("skipping lock", "path", newBkPath)
		return nil
	}

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
			f.Midln(summary.RepoRecordsFromPath(ctx, d.Console(), dbs[i]))
		}
		f.Flush()

		msg := fmt.Sprintf("%s %d item/s", c.Palette().BrightRed.Sprint("removing"), n)
		if err := c.ConfirmErr(ctx, msg+", continue?", "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMessage("removing database..."),
		rotato.WithMessageColor(rotato.FgYellow),
	)
	sp.Start(ctx)

	rmRepo := func(p string) error {
		if err := files.Remove(p); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	}

	for i := range n {
		if err := rmRepo(dbs[i]); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp.Done()

	fmt.Fprintln(d.Writer(), c.SuccessMesg(fmt.Sprintf("%d item/s removed", n)))

	return nil
}

// passwordConfirm prompts user for password input.
func passwordConfirm(ctx context.Context, c *ui.Console) (string, error) {
	s, err := c.InputPassword(ctx, "Password: ")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	fmt.Fprintln(c.Writer())

	s2, err := c.InputPassword(ctx, "Confirm Password: ")
	if err != nil {
		return "", fmt.Errorf("%w", err)
	}

	fmt.Fprintln(c.Writer())

	if s != s2 {
		return "", locker.ErrPassphraseMismatch
	}

	return s, nil
}
