package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// RemoveRepo removes a repo.
func RemoveRepo(d *deps.Deps) error {
	app, err := d.Application()
	if err != nil {
		return err
	}
	if !files.Exists(app.Path.Database) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, app.Path.Database)
	}

	c, p := d.Console(), d.Console().Palette()
	if filepath.Base(app.Path.Database) == application.MainDBName && !app.Flags.Force {
		f := p.BrightYellow.With(ansi.Italic).Sprint("--force")
		return fmt.Errorf("%w: removing the main database requires %s", ErrInvalidOption, f)
	}

	if !app.Flags.Force && !app.Flags.Yes {
		title := p.BrightRed.With(p.Bold).Sprint("Removing Database/s")
		subtitle := p.Dim.With(p.Italic).Sprint("this action cannot be undone")

		c.Frame().Headerln(title).
			Headerln(subtitle).
			Rowln().Flush()

		fmt.Fprint(d.Writer(), summary.RepoFromPath(d, app.Path.Database, app.Path.Backup))
		err := c.ConfirmErr(p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(app.Path.Database)+"?", "n")
		if err != nil {
			return err
		}
	}

	if err := RemoveBackups(d); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(app.Path.Database); err != nil {
		return err
	}

	dbName := files.StripSuffixes(filepath.Base(app.Path.Database))
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(d *deps.Deps) error {
	app, err := d.Application()
	if err != nil {
		return err
	}

	fn := app.Path.Database
	dbName := files.StripSuffixes(filepath.Base(fn))
	fs, err := files.List(app.Path.Backup, "*_"+dbName+".db*") // match YYYYMMDD-HHMMSS_dbname.db
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	if app.Flags.Yes || app.Flags.Force {
		return removeSlicePath(d, fs)
	}

	filesToRemove := make([]string, 0, len(fs))
	c, p := d.Console(), d.Console().Palette()
actionLoop:
	for {
		opt, err := c.Choose(p.BrightRed.Wrap("remove", p.Bold)+" backups?", []string{"all", "no", "select"}, "n")
		if err != nil {
			return err
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			c.ReplaceLine(c.Warning(p.BrightYellow.Sprint("skipping") + " backup/s").StringReset())
			break actionLoop
		case "a", "all":
			filesToRemove = append(filesToRemove, fs...)
			break actionLoop
		case "s", "select":
			c.SetReader(os.Stdin)
			c.SetWriter(os.Stdout)

			selected, err := selection(
				fs,
				func(p *string) string { return summary.BackupWithFmtDateFromPath(d.Context(), d.Console(), *p) },
				menu.WithArgs("--cycle"),
				menu.WithConfig(app.Menu),
				menu.WithMultiSelection(),
				menu.WithOutputColor(app.Flags.Color),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(fn))),
				menu.WithPreview(app.Cmd+" db --db=./backup/{1} info"),
			)
			if err != nil {
				return err
			}
			c.ClearLine(1)
			filesToRemove = append(filesToRemove, selected...)
			break actionLoop
		}
	}

	c.Frame().Headerln(p.BrightRed.Sprint("Removing") + " backups").Rowln().Flush()
	return removeSlicePath(d, filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(d *deps.Deps, dbs []string) error {
	c := d.Console()
	f := c.Frame()
	n := len(dbs)
	if n == 0 {
		return ErrNoItems
	}

	app, err := d.Application()
	if err != nil {
		return err
	}
	if n > 1 && !app.Flags.Yes {
		for i := range n {
			f.Midln(summary.RepoRecordsFromPath(d.Context(), d.Console(), dbs[i]))
		}
		f.Flush()

		msg := fmt.Sprintf("%s %d item/s", c.Palette().BrightRed.Sprint("removing"), n)
		if err := c.ConfirmErr(msg+", continue?", "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	sp := rotato.New(
		rotato.WithMessage("removing database..."),
		rotato.WithMessageColor(rotato.FgYellow),
	)
	sp.Start()

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

	fmt.Println(c.SuccessMesg(fmt.Sprintf("%d item/s removed", n)))

	return nil
}

// Remove prompts the user the records to remove.
func Remove(d *deps.Deps, bs []*bookmark.Bookmark) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	defer r.Close()
	app, err := d.Application()
	if err != nil {
		return err
	}
	if err := validateRemove(bs, app.Flags.Force); err != nil {
		return err
	}

	if app.Flags.Force || app.Flags.Yes {
		return removeRecords(d, bs)
	}

	c := d.Console()
	p := c.Palette()

	title := p.BrightRed.With(p.Bold).
		Sprint("Removing Bookmarks")
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	c.Frame().Headerln(title + comment).
		Headerln(subtitle).
		Rowln().Flush()

	t := d.Console().Term()
	defer t.CancelInterruptHandler()

	items := make([]bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		items = append(items, *bs[i])
	}

	items, err = confirmRemove(d, items)
	if err != nil {
		return err
	}

	toRemove := make([]*bookmark.Bookmark, 0, len(items))
	for i := range items {
		toRemove = append(toRemove, &items[i])
	}

	return removeRecords(d, toRemove)
}

// DropDatabase drops a database.
func DropDatabase(d *deps.Deps) error {
	r, err := d.Repository()
	if err != nil {
		return err
	}
	c := d.Console()
	app, err := d.Application()
	if err != nil {
		return err
	}
	if app.Flags.Yes || app.Flags.Force {
		fmt.Println(c.SuccessMesg("database dropped"))

		return r.DropSecure(d.Context())
	}

	f, p := c.Frame(), c.Palette()
	title := p.BrightRed.
		Wrap("Dropping All Records", p.Bold)
	subtitle := p.Dim.With(p.Italic).
		Sprint("this action cannot be undone")
	comment := p.Dim.With(p.Italic).
		Sprint(" (ctrl-c to exit)")

	s, err := summary.Info(d)
	if err != nil {
		return err
	}

	f.Headerln(title + comment).
		Headerln(subtitle).
		Rowln().
		Text(s).
		Rowln().Flush()

	q := "continue?"
	if r.Name() == application.MainDBName {
		q = c.WarningMesg("dropping \"main\" database, continue?")
	}

	if err := c.ConfirmErr(q, "n"); err != nil {
		return err
	}

	if err := r.DropSecure(d.Context()); err != nil {
		return err
	}

	return c.Term().Print(d.Context(), c.SuccessMesg("database dropped\n"))
}
