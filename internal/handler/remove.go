package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// RemoveRepo removes a repo.
func RemoveRepo(d *deps.Deps) error {
	if !files.Exists(d.Cfg.DBPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, d.Cfg.DBPath)
	}

	if filepath.Base(d.Cfg.DBPath) == config.MainDBName && !d.Cfg.Flags.Force {
		f := ansi.BrightYellow.With(ansi.Italic).Sprint("--force")
		return fmt.Errorf("%w: main database cannot be removed, use %s", ErrInvalidOption, f)
	}

	c, p := d.Console(), d.Console().Palette()
	fmt.Fprint(d.Writer(), summary.RepoFromPath(d, d.Cfg.DBPath, d.Cfg.Path.Backup))
	if !d.Cfg.Flags.Force {
		if err := c.ConfirmErr(
			p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(d.Cfg.DBPath)+"?",
			"n",
		); err != nil {
			return err
		}
	}

	if err := RemoveBackups(d); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(d.Cfg.DBPath); err != nil {
		return err
	}

	dbName := filepath.Base(d.Cfg.DBPath)
	if dbName == config.MainDBName {
		dbName = "main"
	}

	fmt.Fprintln(d.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(d *deps.Deps) error {
	fn := d.Cfg.DBPath
	dbName := files.StripSuffixes(filepath.Base(fn))
	// match YYYYMMDD-HHMMSS_dbname.db
	fs, err := files.List(d.Cfg.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	if d.Cfg.Flags.Yes || d.Cfg.Flags.Force {
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
				menu.WithConfig(d.Cfg.Menu),
				menu.WithMultiSelection(),
				menu.WithOutputColor(d.Cfg.Flags.Color),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(fn))),
				menu.WithPreview(d.Cfg.Cmd+" db --db=./backup/{1} info"),
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

	if n > 1 && !d.Cfg.Flags.Yes {
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
		rotato.WithMesg("removing database..."),
		rotato.WithMesgColor(rotato.ColorYellow),
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
	defer d.DB.Close()
	if err := validateRemove(bs, d.Cfg.Flags.Force); err != nil {
		return err
	}

	if d.Cfg.Flags.Force {
		return removeRecords(d, bs)
	}

	c := d.Console()
	f := frame.New(frame.WithColorBorder(ansi.BrightBlack))
	if !d.Cfg.Flags.Yes {
		f.Header(c.Palette().BrightRed.Sprint("Removing Bookmarks\n\n")).Flush()
	}

	t := d.Console().Term()
	defer t.CancelInterruptHandler()

	m := menu.New[bookmark.Bookmark](
		menu.WithOutputColor(d.Cfg.Flags.Color),
		menu.WithInterruptFn(t.InterruptFn),
		menu.WithMultiSelection(),
	)

	items := make([]bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		items = append(items, *bs[i])
	}

	items, err := confirmRemove(d, m, items)
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

	if !d.Cfg.Flags.Yes {
		q := "continue?"
		if d.DB.Name() == config.MainDBName {
			q = c.WarningMesg("dropping \"main\" database, continue?")
		}

		if err := c.ConfirmErr(q, "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := d.DB.DropSecure(d.Context()); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(c.SuccessMesg("database dropped"))

	return nil
}
