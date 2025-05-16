package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/encryptor"
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
		r, err := selectRepo("select a database from which you want to remove a backup")
		if err != nil {
			return fmt.Errorf("backup: %w", err)
		}
		r.Close()
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		m := menu.New[string](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithMultiSelection(),
			menu.WithHeader("choose backup/s to remove", false),
			menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		fs, err := r.BackupsList()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		// backups := slice.New[string]()
		// backups.Append(fs...)
		items, err := handler.Selection(m, fs, func(s *string) string {
			return repo.BackupSummaryWithFmtDateFromPath(*s)
		})
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		backups := slice.New[string]()
		backups.Set(&items)
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		rm := color.BrightRed("Removing").String()
		f.Header(rm + " backup/s from " + r.Name()).Ln().Row("\n")

		backups.ForEach(func(s string) {
			f.Mid(repo.RepoSummaryRecordsFromPath(s)).Ln()
		})

		rm = color.BrightRed("remove").Bold().String()
		msg := fmt.Sprintf(rm+" %d backup/s?", backups.Len())
		if err := t.ConfirmErr(f.Row("\n").Question(msg).String(), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := backups.ForEachErr(files.Remove); err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}
		success := color.BrightGreen("Successfully").Italic().String()
		t.ReplaceLine(1, f.Clear().Success(success+" backup/s removed").String())

		return nil
	},
}

// dbRemoveCmd remove a database.
var dbRemoveCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db", "d"},
	Short:   "Remove a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		var repoPath string
		var err error
		if Menu {
			repoPath, err = handler.SelectionRepo(args)
			if err != nil {
				return fmt.Errorf("failed to select repo: %w", err)
			}
		} else {
			repoPath = Cfg.Fullpath()
		}
		if err := encryptor.IsEncrypted(repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		cfg, err := repo.NewSQLiteCfg(repoPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		r, err := repo.New(cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		// show repo info
		i := repo.Info(r)
		i += f.Row().Ln().String()
		fmt.Print(i)

		rm := color.BrightRed("remove").Bold().String()
		if err := t.ConfirmErr(f.Clear().Question(rm+" "+r.Name()+"?").String(), "y"); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := handler.RemoveBackups(t, f.Clear(), r); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := files.Remove(r.Cfg.Fullpath()); err != nil {
			return fmt.Errorf("%w", err)
		}

		success := color.BrightGreen("Successfully").Italic().String()
		t.ReplaceLine(1, f.Clear().Success(success+" removed database").String())

		return nil
	},
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
