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
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	cred  = func(s string) string { return color.BrightRed(s).String() }        // BrightRed
	credB = func(s string) string { return color.BrightRed(s).Bold().String() } // BrightRed/Bold
	cyel  = func(s string) string { return color.BrightYellow(s).String() }     // BrightYellow
)

// RemoveRepo removes a repo.
func RemoveRepo(a *app.Context) error {
	if !files.Exists(a.Cfg.DBPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, a.Cfg.DBPath)
	}

	if filepath.Base(a.Cfg.DBPath) == config.MainDBName && !a.Cfg.Flags.Force {
		return fmt.Errorf("%w: main database cannot be removed, use --force", sys.ErrActionAborted)
	}

	fmt.Print(summary.RepoFromPath(a, a.Cfg.DBPath, a.Cfg.Path.Backup))
	if !a.Cfg.Flags.Force {
		if err := a.Console.ConfirmErr(credB("remove")+" "+filepath.Base(a.Cfg.DBPath)+"?", "n"); err != nil {
			return err
		}
	}

	if err := RemoveBackups(a); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.Remove(a.Cfg.DBPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	dbName := filepath.Base(a.Cfg.DBPath)
	if dbName == config.MainDBName {
		dbName = "main"
	}

	fmt.Print(a.Console.SuccessMesg("database " + dbName + " removed\n"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(a *app.Context) error {
	p := a.Cfg.DBPath
	dbName := files.StripSuffixes(filepath.Base(p))
	fs, err := files.List(a.Cfg.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	filesToRemove := slice.New[string]()

	if a.Cfg.Flags.Force {
		filesToRemove.Append(fs...)
		return removeSlicePath(a, filesToRemove)
	}

actionLoop:
	for {
		opt, err := a.Console.Choose(credB("remove")+" backups?", []string{"all", "no", "select"}, "n")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			a.Console.ReplaceLine(a.Console.Warning(cyel("skipping") + " backup/s").StringReset())
			break actionLoop
		case "a", "all":
			filesToRemove.Append(fs...)
			break actionLoop
		case "s", "select":
			a.Console.SetReader(os.Stdin)
			a.Console.SetWriter(os.Stdout)

			selected, err := selection(fs,
				func(p *string) string { return summary.BackupWithFmtDateFromPath(a.Ctx, *p) },
				menu.WithArgs("--cycle"),
				menu.WithSettings(a.Cfg.Menu.Settings),
				menu.WithMultiSelection(),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(p)), false),
				menu.WithPreview(a.Cfg.Cmd+" db --name=./backup/{1} --info"),
			)

			if errors.Is(err, sys.ErrActionAborted) {
				continue
			}

			if err != nil {
				return fmt.Errorf("%w", err)
			}
			a.Console.ClearLine(1)
			filesToRemove.Append(selected...)
			break actionLoop
		}
	}

	return removeSlicePath(a, filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(a *app.Context, dbs *slice.Slice[string]) error {
	n := dbs.Len()
	if n == 0 {
		return slice.ErrSliceEmpty
	}

	if n > 1 && !a.Cfg.Flags.Yes {
		dbs.ForEach(func(r string) {
			a.Console.Frame.Midln(summary.RepoRecordsFromPath(a.Ctx, r))
		})

		a.Console.Frame.Flush()

		msg := fmt.Sprintf("%s %d item/s", cred("removing"), n)
		if err := a.Console.ConfirmErr(msg+", continue?", "n"); err != nil {
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

	fmt.Print(a.Console.SuccessMesg(fmt.Sprintf("%d item/s removed\n", dbs.Len())))

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

	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(cred("Removing Bookmarks\n\n")).Flush()

	defer a.Console.Term.CancelInterruptHandler()

	m := menu.New[bookmark.Bookmark](
		menu.WithInterruptFn(a.Console.Term.InterruptFn),
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
	a.Console.Frame.Header(cred("Dropping") + " all records\n").Row("\n").Flush()
	fmt.Print(summary.Info(a))

	a.Console.Frame.Reset().Rowln().Flush()

	if !a.Cfg.Flags.Yes {
		q := "continue?"
		if a.DB.Name() == config.MainDBName {
			q = a.Console.WarningMesg("dropping \"main\" database, continue?")
		}

		if err := a.Console.ConfirmErr(q, "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := a.DB.DropSecure(a.Ctx); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(a.Console.SuccessMesg("database dropped\n"))

	return nil
}
