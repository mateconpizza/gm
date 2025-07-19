package handler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/summary"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/repository"
)

var (
	cred  = func(s string) string { return color.BrightRed(s).String() }
	credB = func(s string) string { return color.BrightRed(s).Bold().String() }
	cyel  = func(s string) string { return color.BrightYellow(s).String() }
)

// RemoveRepo removes a repo.
func RemoveRepo(c *ui.Console, dbPath string) error {
	if !files.Exists(dbPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, dbPath)
	}

	if filepath.Base(dbPath) == config.MainDBName && !config.App.Flags.Force {
		return fmt.Errorf("%w: main database cannot be removed, use --force", terminal.ErrActionAborted)
	}

	fmt.Print(summary.RepoFromPath(c, dbPath))

	if !config.App.Flags.Force {
		if err := c.ConfirmErr(credB("remove")+" "+filepath.Base(dbPath)+"?", "n"); err != nil {
			return err
		}
	}

	if err := RemoveBackups(c, dbPath); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.Remove(dbPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	dbName := filepath.Base(dbPath)
	if dbName == config.MainDBName {
		dbName = "main"
	}

	fmt.Print(c.SuccessMesg("database " + dbName + " removed\n"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(c *ui.Console, p string) error {
	fs, err := db.ListBackups(config.App.Path.Backup, files.StripSuffixes(filepath.Base(p)))
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	filesToRemove := slice.New[string]()

	if config.App.Flags.Force {
		filesToRemove.Append(fs...)
		return removeSlicePath(c, filesToRemove)
	}

actionLoop:
	for {
		opt, err := c.Choose(credB("remove")+" backups?", []string{"all", "no", "select"}, "n")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			c.ReplaceLine(c.Warning(cyel("skipping") + " backup/s").StringReset())
			break actionLoop
		case "a", "all":
			filesToRemove.Append(fs...)
			break actionLoop
		case "s", "select":
			c.SetReader(os.Stdin)
			c.SetWriter(os.Stdout)

			selected, err := selection(fs,
				func(p *string) string { return summary.BackupWithFmtDateFromPath(*p) },
				menu.WithArgs("--cycle"),
				menu.WithUseDefaults(),
				menu.WithSettings(config.Fzf.Settings),
				menu.WithMultiSelection(),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(p)), false),
				menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
			)

			if errors.Is(err, sys.ErrActionAborted) {
				continue
			}

			if err != nil {
				return fmt.Errorf("%w", err)
			}
			c.ClearLine(1)
			filesToRemove.Append(selected...)
			break actionLoop
		}
	}

	return removeSlicePath(c, filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(c *ui.Console, dbs *slice.Slice[string]) error {
	n := dbs.Len()
	if n == 0 {
		return slice.ErrSliceEmpty
	}

	if n > 1 && !config.App.Flags.Force {
		dbs.ForEach(func(r string) {
			c.F.Midln(summary.RepoRecordsFromPath(r))
		})

		c.F.Flush()

		msg := fmt.Sprintf("%s %d item/s", cred("removing"), n)
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

	fmt.Print(c.SuccessMesg(fmt.Sprintf("%d item/s removed\n", dbs.Len())))

	return nil
}

// Remove prompts the user the records to remove.
func Remove(c *ui.Console, r repository.Repo, bs []*bookmark.Bookmark) error {
	defer r.Close()
	if err := validateRemove(bs, config.App.Flags.Force); err != nil {
		return err
	}

	if config.App.Flags.Force {
		return removeRecords(c, r, bs)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(cred("Removing Bookmarks\n\n")).Flush()

	defer c.T.CancelInterruptHandler()

	m := menu.New[bookmark.Bookmark](
		menu.WithInterruptFn(c.T.InterruptFn),
		menu.WithMultiSelection(),
	)

	// FIX: use []*bookmark.Bookmark
	fixMe := slice.New[bookmark.Bookmark]()
	for i := range bs {
		fixMe.Push(bs[i])
	}

	if err := confirmRemove(c, m, fixMe); err != nil {
		return err
	}

	return removeRecords(c, r, bs)
}

// DroppingDB drops a database.
func DroppingDB(c *ui.Console, r *db.SQLite) error {
	c.F.Header(cred("Dropping") + " all records\n").Row("\n").Flush()
	repo := repository.New(r)
	fmt.Print(summary.Info(c, repo))

	c.F.Reset().Rowln().Flush()

	if !config.App.Flags.Force {
		var q string
		if r.Cfg.Name == config.MainDBName {
			q = c.WarningMesg("dropping \"main\" database, continue?")
		} else {
			q = "continue?"
		}

		if err := c.ConfirmErr(q, "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := r.DropSecure(context.Background()); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(c.SuccessMesg("database dropped\n"))

	return nil
}
