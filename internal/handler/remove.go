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
	if !files.Exists(d.App.Path.Database) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, d.App.Path.Database)
	}

	c, p := d.Console(), d.Console().Palette()
	if filepath.Base(d.App.Path.Database) == application.MainDBName && !d.App.Flags.Force {
		f := p.BrightYellow.With(ansi.Italic).Sprint("--force")
		return fmt.Errorf("%w: removing the main database requires %s", ErrInvalidOption, f)
	}

	if !d.App.Flags.Force && !d.App.Flags.Yes {
		title := p.BrightRed.With(p.Bold).Sprint("Removing Database/s")
		subtitle := p.Dim.With(p.Italic).Sprint("this action cannot be undone")

		c.Frame().Headerln(title).
			Headerln(subtitle).
			Rowln().Flush()

		fmt.Fprint(d.Writer(), summary.RepoFromPath(d, d.App.Path.Database, d.App.Path.Backup))
		err := c.ConfirmErr(p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(d.App.Path.Database)+"?", "n")
		if err != nil {
			return err
		}
	}

	if err := RemoveBackups(d); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(d.App.Path.Database); err != nil {
		return err
	}

	dbName := files.StripSuffixes(filepath.Base(d.App.Path.Database))
	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(d *deps.Deps) error {
	fn := d.App.Path.Database
	dbName := files.StripSuffixes(filepath.Base(fn))
	// match YYYYMMDD-HHMMSS_dbname.db
	fs, err := files.List(d.App.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	if d.App.Flags.Yes || d.App.Flags.Force {
		return removeSlicePath(d, fs)
	}

	c, p := d.Console(), d.Console().Palette()

	filesToRemove := make([]string, 0, len(fs))

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

			selected, err := selection(fs,
				func(p *string) string { return summary.BackupWithFmtDateFromPath(d.Context(), d.Console(), *p) },
				menu.WithArgs("--cycle"),
				menu.WithConfig(d.App.Menu),
				menu.WithMultiSelection(),
				menu.WithOutputColor(d.App.Flags.Color),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(fn))),
				menu.WithPreview(d.App.Cmd+" db --db=./backup/{1} info"),
			)
			if err != nil {
				return err
			}
			c.ClearLine(1)
			filesToRemove = append(filesToRemove, selected...)
			break actionLoop
		}
	}

	c.Frame().Headerln(c.Palette().BrightRed.Sprint("Removing") + " backups").Rowln().Flush()

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

	if n > 1 && !d.App.Flags.Yes {
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
	defer d.Repo.Close()
	if err := validateRemove(bs, d.App.Flags.Force); err != nil {
		return err
	}

	if d.App.Flags.Force {
		return removeRecords(d, bs)
	}

	c, p := d.Console(), d.Console().Palette()

	if !d.App.Flags.Yes {
		f := c.Frame()
		title := p.BrightRed.With(p.Bold).
			Sprint("Removing Bookmarks")
		subtitle := p.Dim.With(p.Italic).
			Sprint("this action cannot be undone")
		comment := p.Dim.With(p.Italic).
			Sprint(" (ctrl-c to exit)")

		f.Headerln(title + comment).
			Headerln(subtitle).
			Rowln().Flush()
	}

	t := d.Console().Term()
	defer t.CancelInterruptHandler()

	items := make([]bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		items = append(items, *bs[i])
	}

	items, err := confirmRemove(d, items)
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
	c, f := d.Console(), d.Console().Frame()
	f.Header(c.Palette().BrightRed.Sprint("Dropping") + " all records\n").Row("\n").Flush()
	fmt.Print(summary.Info(d))

	f.Reset().Rowln().Flush()

	if !d.App.Flags.Yes {
		q := "continue?"
		if d.Repo.Name() == application.MainDBName {
			q = c.WarningMesg("dropping \"main\" database, continue?")
		}

		if err := c.ConfirmErr(q, "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := d.Repo.DropSecure(d.Context()); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(c.SuccessMesg("database dropped"))

	return nil
}
