package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

// RemoveRepo removes a repo.
func RemoveRepo(a *app.Context) error {
	if !files.Exists(a.Cfg.DBPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, a.Cfg.DBPath)
	}

	if filepath.Base(a.Cfg.DBPath) == config.MainDBName && !a.Cfg.Flags.Force {
		return fmt.Errorf("%w: main database cannot be removed, use --force", sys.ErrActionAborted)
	}

	c, p := a.Console(), a.Console().Palette()
	fmt.Fprint(a.Writer(), summary.RepoFromPath(a, a.Cfg.DBPath, a.Cfg.Path.Backup))
	if !a.Cfg.Flags.Force {
		if err := c.ConfirmErr(p.BrightRed.Wrap("remove", p.Bold)+" "+filepath.Base(a.Cfg.DBPath)+"?", "n"); err != nil {
			return err
		}
	}

	if err := RemoveBackups(a); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return err
		}
	}

	if err := files.Remove(a.Cfg.DBPath); err != nil {
		return err
	}

	dbName := filepath.Base(a.Cfg.DBPath)
	if dbName == config.MainDBName {
		dbName = "main"
	}

	fmt.Fprintln(a.Writer(), c.SuccessMesg("database "+dbName+" removed"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(a *app.Context) error {
	fn := a.Cfg.DBPath
	dbName := files.StripSuffixes(filepath.Base(fn))
	// match YYYYMMDD-HHMMSS_dbname.db
	fs, err := files.List(a.Cfg.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return err
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	filesToRemove := slice.New[string]()
	if a.Cfg.Flags.Yes {
		filesToRemove.Append(fs...)
		return removeSlicePath(a, filesToRemove)
	}

	c, p := a.Console(), a.Console().Palette()

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
			filesToRemove.Append(fs...)
			break actionLoop
		case "s", "select":
			c.SetReader(os.Stdin)
			c.SetWriter(os.Stdout)

			selected, err := selection(fs,
				func(p *string) string { return summary.BackupWithFmtDateFromPath(a.Context(), a.Console(), *p) },
				menu.WithArgs("--cycle"),
				menu.WithConfig(a.Cfg.Menu),
				menu.WithMultiSelection(),
				menu.WithOutputColor(a.Cfg.Flags.Color),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(fn))),
				menu.WithPreview(a.Cfg.Cmd+" db --name=./backup/{1} --info"),
			)
			if err != nil {
				return err
			}
			c.ClearLine(1)
			filesToRemove.Append(selected...)
			break actionLoop
		}
	}

	c.Frame().Headerln(c.Palette().BrightRed.Sprint("Removing") + " backups").Rowln().Flush()

	return removeSlicePath(a, filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(a *app.Context, dbs *slice.Slice[string]) error {
	c := a.Console()
	f := c.Frame()
	n := dbs.Len()
	if n == 0 {
		return slice.ErrSliceEmpty
	}

	if n > 1 && !a.Cfg.Flags.Yes {
		dbs.ForEach(func(r string) {
			f.Midln(summary.RepoRecordsFromPath(a.Context(), a.Console(), r))
		})

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

	if err := dbs.ForEachErr(rmRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()

	fmt.Println(c.SuccessMesg(fmt.Sprintf("%d item/s removed", dbs.Len())))

	return nil
}

// Remove prompts the user the records to remove.
func Remove(a *app.Context, bs []*bookmark.Bookmark) error {
	defer a.DB.Close()
	if err := validateRemove(bs, a.Cfg.Flags.Force); err != nil {
		return err
	}

	if a.Cfg.Flags.Force {
		return removeRecords(a, bs)
	}

	c := a.Console()
	f := frame.New(frame.WithColorBorder(ansi.BrightBlack))
	f.Header(c.Palette().BrightRed.Sprint("Removing Bookmarks\n\n")).Flush()

	t := a.Console().Term()
	defer t.CancelInterruptHandler()

	m := menu.New[bookmark.Bookmark](
		menu.WithOutputColor(a.Cfg.Flags.Color),
		menu.WithInterruptFn(t.InterruptFn),
		menu.WithMultiSelection(),
	)

	// FIX: use []*bookmark.Bookmark
	fixMe := slice.New[bookmark.Bookmark]()
	for i := range bs {
		fixMe.Push(bs[i])
	}

	if err := confirmRemove(a, m, fixMe); err != nil {
		return err
	}

	return removeRecords(a, bs)
}

// DroppingDB drops a database.
func DroppingDB(a *app.Context) error {
	c := a.Console()
	f := c.Frame()
	f.Header(c.Palette().BrightRed.Sprint("Dropping") + " all records\n").Row("\n").Flush()
	fmt.Print(summary.Info(a))

	f.Reset().Rowln().Flush()

	if !a.Cfg.Flags.Yes {
		q := "continue?"
		if a.DB.Name() == config.MainDBName {
			q = c.WarningMesg("dropping \"main\" database, continue?")
		}

		if err := c.ConfirmErr(q, "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := a.DB.DropSecure(a.Context()); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(c.SuccessMesg("database dropped"))

	return nil
}
