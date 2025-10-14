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
	"github.com/mateconpizza/gm/internal/ui"
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
func RemoveRepo(c *ui.Console, cfg *config.Config) error {
	if !files.Exists(cfg.DBPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, cfg.DBPath)
	}

	if filepath.Base(cfg.DBPath) == config.MainDBName && !cfg.Flags.Force {
		return fmt.Errorf("%w: main database cannot be removed, use --force", sys.ErrActionAborted)
	}

	fmt.Print(summary.RepoFromPath(c, cfg.DBPath, cfg.Path.Backup))
	if !cfg.Flags.Force {
		if err := c.ConfirmErr(credB("remove")+" "+filepath.Base(cfg.DBPath)+"?", "n"); err != nil {
			return err
		}
	}

	if err := RemoveBackups(c, cfg); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.Remove(cfg.DBPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	dbName := filepath.Base(cfg.DBPath)
	if dbName == config.MainDBName {
		dbName = "main"
	}

	fmt.Print(c.SuccessMesg("database " + dbName + " removed\n"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(c *ui.Console, cfg *config.Config) error {
	p := cfg.DBPath
	dbName := files.StripSuffixes(filepath.Base(p))
	fs, err := files.List(cfg.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	filesToRemove := slice.New[string]()

	if cfg.Flags.Force {
		filesToRemove.Append(fs...)
		return removeSlicePath(c, filesToRemove, cfg.Flags.Force)
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
				menu.WithSettings(config.Fzf.Settings),
				menu.WithMultiSelection(),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(p)), false),
				menu.WithPreview(cfg.Cmd+" db -n ./backup/{1} info"),
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

	return removeSlicePath(c, filesToRemove, cfg.Flags.Force)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(c *ui.Console, dbs *slice.Slice[string], force bool) error {
	n := dbs.Len()
	if n == 0 {
		return slice.ErrSliceEmpty
	}

	if n > 1 && !force {
		dbs.ForEach(func(r string) {
			c.Frame.Midln(summary.RepoRecordsFromPath(r))
		})

		c.Frame.Flush()

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
func Remove(c *ui.Console, r *db.SQLite, bs []*bookmark.Bookmark, cfg *config.Config) error {
	defer r.Close()
	if err := validateRemove(bs, cfg.Flags.Force); err != nil {
		return err
	}

	if cfg.Flags.Force {
		return removeRecords(c, r, cfg, bs)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	f.Header(cred("Removing Bookmarks\n\n")).Flush()

	defer c.Term.CancelInterruptHandler()

	m := menu.New[bookmark.Bookmark](
		menu.WithInterruptFn(c.Term.InterruptFn),
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

	return removeRecords(c, r, cfg, bs)
}

// DroppingDB drops a database.
func DroppingDB(c *ui.Console, r *db.SQLite, backupPath string, force bool) error {
	c.Frame.Header(cred("Dropping") + " all records\n").Row("\n").Flush()
	fmt.Print(summary.Info(c, r, backupPath))

	c.Frame.Reset().Rowln().Flush()

	if !force {
		q := "continue?"
		if r.Name() == config.MainDBName {
			q = c.WarningMesg("dropping \"main\" database, continue?")
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
