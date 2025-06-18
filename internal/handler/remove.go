package handler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

// RemoveRepo removes a repo.
func RemoveRepo(t *terminal.Term, dbPath string) error {
	if !files.Exists(dbPath) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, dbPath)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if filepath.Base(dbPath) == config.DefaultDBName && !config.App.Force {
		return fmt.Errorf("%w: default database cannot be removed, use --force", terminal.ErrActionAborted)
	}
	fmt.Print(db.RepoSummaryFromPath(f.Reset(), dbPath))

	if !config.App.Force {
		rm := color.BrightRed("remove").Bold().String()
		if err := t.ConfirmErr(f.Reset().Row("\n").Question(rm+" "+filepath.Base(dbPath)+"?").String(), "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := RemoveBackups(t, f.Reset(), dbPath); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.Remove(dbPath); err != nil {
		return fmt.Errorf("%w", err)
	}

	if GitInitialized(config.App.Path.Git, dbPath) {
		g, err := NewGit(config.App.Path.Git)
		if err != nil {
			return err
		}
		g.Tracker.SetCurrent(g.NewRepo(dbPath))
		if err := GitDropRepo(g, "Dropped"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	dbName := filepath.Base(dbPath)
	if dbName == config.DefaultDBName {
		dbName = "default"
	}

	fmt.Print(txt.SuccessMesg("database " + dbName + " removed\n"))

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(t *terminal.Term, f *frame.Frame, p string) error {
	fs, err := db.ListBackups(config.App.Path.Backup, filepath.Base(p))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}

	filesToRemove := slice.New[string]()

	if !config.App.Force {
	actionLoop:
		for {
			rm := color.BrightRed("remove").Bold().String()
			f.Question(rm + " backups?")
			opt, err := t.Choose(f.String(), []string{"all", "no", "select"}, "n")
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			f.Reset()

			switch strings.ToLower(opt) {
			case "n", "no":
				sk := color.BrightYellow("skipping").String()
				t.ReplaceLine(1, f.Reset().Warning(sk+" backup/s\n").Row().String())

				break actionLoop
			case "a", "all":
				filesToRemove.Append(fs...)
				break actionLoop
			case "s", "select":
				selected, err := selection(fs,
					func(p *string) string { return db.BackupSummaryWithFmtDateFromPath(*p) },
					menu.WithArgs("--cycle"),
					menu.WithUseDefaults(),
					menu.WithSettings(config.Fzf.Settings),
					menu.WithMultiSelection(),
					menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(p)), false),
					menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
				)
				if err != nil && !errors.Is(err, sys.ErrActionAborted) {
					return fmt.Errorf("%w", err)
				}
				t.ClearLine(1)
				filesToRemove.Append(selected...)
			}
		}
	} else {
		filesToRemove.Append(fs...)
	}

	return removeSlicePath(f.Reset(), filesToRemove)
}

// removeSlicePath removes a slice of paths.
func removeSlicePath(f *frame.Frame, dbs *slice.Slice[string]) error {
	n := dbs.Len()
	if n == 0 {
		return slice.ErrSliceEmpty
	}
	t := terminal.New(terminal.WithInterruptFn(func(err error) { sys.ErrAndExit(err) }))
	s := color.BrightRed("removing").String()
	if n > 1 && !config.App.Force {
		f.Row("\n")
		dbs.ForEach(func(r string) {
			f.Mid(db.RepoSummaryRecordsFromPath(r)).Ln()
		})

		msg := fmt.Sprintf("%s %d item/s", s, n)
		if err := t.ConfirmErr(f.Row("\n").Question(msg+", continue?").String(), "n"); err != nil {
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

	fmt.Print(txt.SuccessMesg(fmt.Sprintf("%d item/s removed\n", dbs.Len())))

	return nil
}

// Remove prompts the user the records to remove.
func Remove(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	defer r.Close()
	if err := validateRemove(bs, config.App.Force); err != nil {
		return err
	}

	if !config.App.Force {
		cbr := func(s string) string { return color.BrightRed(s).String() }
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Header(cbr("Removing Bookmarks\n\n")).Flush()

		interruptFn := func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}

		t := terminal.New(terminal.WithInterruptFn(interruptFn))
		defer t.CancelInterruptHandler()

		m := menu.New[bookmark.Bookmark](
			menu.WithInterruptFn(interruptFn),
			menu.WithMultiSelection(),
		)

		s := color.BrightRed("remove").Bold().String()
		if err := confirmRemove(m, t, bs, s); err != nil {
			return err
		}
	}

	return removeRecords(r, bs)
}

// DroppingDB drops a database.
func DroppingDB(t *terminal.Term, r *db.SQLiteRepository) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header(color.BrightRed("Dropping").String() + " all records\n").Row("\n").Flush()
	fmt.Print(db.Info(f, r))

	f.Reset().Rowln().Flush()

	if !config.App.Force {
		if r.Cfg.Name == config.DefaultDBName {
			f.Text(txt.WarningMesg("dropping 'default' database, continue?"))
		} else {
			f.Question("continue?")
		}
		if err := t.ConfirmErr(f.String(), "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := r.DropSecure(context.Background()); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Print(txt.SuccessMesg("database dropped\n"))

	return nil
}
