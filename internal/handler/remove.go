package handler

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mateconpizza/rotato"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/menu"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
)

// RemoveRepo removes a repo.
func RemoveRepo(t *terminal.Term, p string) error {
	pp, err := FindDB(p)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	p = pp
	if !files.Exists(p) {
		return fmt.Errorf("%w: %q", db.ErrDBNotFound, p)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if filepath.Base(p) == config.DefaultDBName && !config.App.Force {
		return fmt.Errorf("%w: default database cannot be removed, use --force", terminal.ErrActionAborted)
	}
	i := db.RepoSummaryFromPath(p)
	fmt.Print(i)

	if !config.App.Force {
		rm := color.BrightRed("remove").Bold().String()
		if err := t.ConfirmErr(f.Row("\n").Question(rm+" "+filepath.Base(p)+"?").String(), "n"); err != nil {
			return fmt.Errorf("%w", err)
		}
	}

	if err := RemoveBackups(t, f.Clear(), p); err != nil {
		if !errors.Is(err, db.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
	}

	if err := files.Remove(p); err != nil {
		return fmt.Errorf("%w", err)
	}

	s := color.BrightGreen("Successfully").Italic().String()
	var a string
	dbName := filepath.Base(p)
	if dbName == config.DefaultDBName {
		a = color.Text(fmt.Sprintf("%q", "main")).Italic().String()
	} else {
		a = color.Text(fmt.Sprintf("%q", filepath.Base(p))).Italic().String()
	}
	f.Clear().Success(s + " database " + a + " removed\n").Flush()

	return nil
}

// RemoveBackups removes backups.
func RemoveBackups(t *terminal.Term, f *frame.Frame, p string) error {
	fs, err := db.ListDatabaseBackups(config.App.Path.Backup, filepath.Base(p))
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if len(fs) == 0 {
		return db.ErrBackupNotFound
	}
	rm := color.BrightRed("remove").Bold().String()
	f.Question(rm + " backups?")

	filesToRemove := slice.New[string]()
	if config.App.Force {
		filesToRemove.Append(fs...)
	} else {
		opt, err := t.Choose(f.String(), []string{"all", "no", "select"}, "n")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		switch strings.ToLower(opt) {
		case "n", "no":
			sk := color.BrightYellow("skipping").String()
			t.ReplaceLine(1, f.Clear().Warning(sk+" backup/s\n").Row().String())
		case "a", "all":
			filesToRemove.Append(fs...)
		case "s", "select":
			selected, err := Select(fs,
				func(p *string) string { return db.BackupSummaryWithFmtDateFromPath(*p) },
				menu.WithArgs("--cycle"),
				menu.WithUseDefaults(),
				menu.WithSettings(config.Fzf.Settings),
				menu.WithMultiSelection(),
				menu.WithHeader(fmt.Sprintf("select backup/s from %q", filepath.Base(p)), false),
				menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
			)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			t.ClearLine(1)
			filesToRemove.Append(selected...)
		}
	}

	return removeSlicePath(f.Clear(), filesToRemove)
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
	s = color.BrightGreen("Successfully").Italic().String()
	f.Clear().Row("\n").Success(s + " " + strconv.Itoa(dbs.Len()) + " item/s removed\n").Flush()

	return nil
}

// Remove prompts the user the records to remove.
func Remove(r *db.SQLiteRepository, bs *slice.Slice[bookmark.Bookmark]) error {
	defer r.Close()
	if err := validateRemove(bs, config.App.Force); err != nil {
		return err
	}
	if !config.App.Force {
		c := color.BrightRed
		f := frame.New(frame.WithColorBorder(color.Gray))
		f.Header(c("Removing Bookmarks\n\n").String()).Flush()

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
	fmt.Print(db.Info(r))

	f.Clear().Rowln().Flush()

	if !config.App.Force {
		if r.Cfg.Name == config.DefaultDBName {
			f.Warning(color.Text("dropping 'default' database, continue?").Bold().String())
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
	success := color.BrightGreen("Successfully").Italic().String()
	f.Clear().Success(success + " database dropped").Ln().Flush()

	return nil
}
