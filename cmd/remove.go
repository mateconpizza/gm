package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/haaag/rotato"
	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// rmRepo removes a repo.
func rmRepo(r *Repo) error {
	if err := files.Remove(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// selectRepo prompts the user to select a repo.
func selectRepo(p string) (*Repo, error) {
	databases, err := repo.Databases(Cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	m := menu.New[Repo](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithHeader(p, false),
		menu.WithPreview(config.App.Cmd+" db -n {1} info"),
	)
	repos, err := handler.Selection(m, *databases.Items(), repo.RepoSummaryRecords)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &repos[0], nil
}

// bkRemoveCmd removes backups.
var bkRemoveCmd = &cobra.Command{
	Use:     "backup",
	Short:   "Remove a backup",
	Aliases: []string{"bk", "b", "backups"},
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := selectRepo("choose a database to remove a backup from")
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		r.Close()
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		m := menu.New[Repo](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithMultiSelection(),
			menu.WithHeader("choose backup/s to remove", false),
			menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		backups, err := repo.Backups(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		rm := color.BrightRed("Removing").String()
		f.Header(rm + " backup/s from '" + r.Cfg.Name).Ln().Row("\n")
		items, err := handler.Selection(m, *backups.Items(), repo.RepoSummaryRecords)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		backups.Set(&items)
		backups.ForEachMut(func(r *Repo) {
			f.Mid(repo.RepoSummaryRecords(r)).Ln()
		})

		rm = color.BrightRed("remove").Bold().String()
		msg := fmt.Sprintf(rm+" %d backup/s?", backups.Len())
		if !t.Confirm(f.Row("\n").Error(msg).String(), "n") {
			return sys.ErrActionAborted
		}

		if err := backups.ForEachMutErr(rmRepo); err != nil {
			return fmt.Errorf("%w", err)
		}
		success := color.BrightGreen("Successfully").Italic().String()
		t.ReplaceLine(1, f.Clear().Success(success+" backup/s removed").String())

		return nil
	},
}

// handleRmBackups removes backups.
func handleRmBackups(t *terminal.Term, r *Repo) error {
	filesToRemove := slice.New[Repo]()
	backups, err := repo.Backups(r)
	if err != nil {
		if !errors.Is(err, repo.ErrBackupNotFound) {
			return fmt.Errorf("%w", err)
		}
		backups = slice.New[Repo]()
	}
	if backups.Len() == 0 {
		return nil
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray))
	rm := color.BrightRed("remove").Bold().String()
	items := backups.Items()
	f.Clear().Mid(rm + " backups?")
	opt := t.Choose(f.String(), []string{"all", "no", "select"}, "n")
	switch strings.ToLower(opt) {
	case "n", "no":
		sk := color.BrightYellow("skipping").String()
		t.ReplaceLine(1, f.Clear().Warning(sk+" backup/s\n").Row().String())
	case "a", "all":
		filesToRemove.Set(items)
	case "s", "select":
		m := menu.New[Repo](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithMultiSelection(),
			menu.WithHeader(fmt.Sprintf("select backup/s from '%s'", r.Cfg.Name), false),
			menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		selected, err := handler.Selection(m, *items, repo.RepoSummaryRecords)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		t.ClearLine(1)
		filesToRemove.Append(selected...)
	}
	filesToRemove.Push(r)

	return rmDatabases(t, filesToRemove)
}

// dbRemoveCmd remove a database.
var dbRemoveCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db", "d"},
	Short:   "Remove a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := selectRepo("select database to remove")
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		if !r.Cfg.Exists() {
			return repo.ErrDBNotFound
		}
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		// show repo info
		i := repo.Info(r)
		i += f.Row().Ln().String()
		fmt.Print(i)

		rm := color.BrightRed("remove").Bold().String()
		if !t.Confirm(f.Clear().Error(rm+" "+r.Cfg.Name+"?").String(), "n") {
			return sys.ErrActionAborted
		}
		if err := handleRmBackups(t, r); err != nil {
			return err
		}
		if err := rmRepo(r); err != nil {
			return fmt.Errorf("%w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		t.ReplaceLine(1, f.Clear().Success(success+" removed database").String())

		return nil
	},
}

// rmDatabases removes a list of databases.
func rmDatabases(t *terminal.Term, dbs *slice.Slice[Repo]) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	s := color.BrightRed("removing").String()
	dbs.ForEachMut(func(r *Repo) {
		f.Info(repo.RepoSummaryRecords(r)).Ln()
	})

	msg := s + " " + strconv.Itoa(dbs.Len()) + " items/s"
	if !t.Confirm(f.Row("\n").Warning(msg+", continue?").String(), "n") {
		return sys.ErrActionAborted
	}

	sp := rotato.New(
		rotato.WithMesg("removing database..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()
	if err := dbs.ForEachMutErr(rmRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Done()
	t.ClearLine(format.CountLines(f.String()))
	s = color.BrightGreen("Successfully").Italic().String()
	f.Clear().Success(s + " " + strconv.Itoa(dbs.Len()) + " items/s removed\n").Flush()

	return nil
}

var removeCmd = &cobra.Command{
	Use:     "remove",
	Short:   "Remove databases/backups",
	Aliases: []string{"rm", "del"},
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Usage()
	},
}

func init() {
	removeCmd.AddCommand(dbRemoveCmd, bkRemoveCmd)
	rootCmd.AddCommand(removeCmd)
}
