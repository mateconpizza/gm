//nolint:wrapcheck //ignore
package imports

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	cmdGit "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
)

var ErrImportSourceNotFound = errors.New("import source not found")

func init() {
	importFromCmd.AddCommand(
		importFromBackupCmd,
		importFromBrowserCmd,
		importFromDatabaseCmd,
		importFromGitRepoCmd,
	)
	cmd.Root.AddCommand(importFromCmd)
}

// imports bookmarks from various sources.
var (
	importFromCmd = &cobra.Command{
		Use:     "import",
		Aliases: []string{"i"},
		Short:   "Import bookmarks from various sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	importFromDatabaseCmd = &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Import bookmarks from a database",
		RunE:    fromDatabaseFunc,
	}

	importFromBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   "Import bookmarks from a backup",
		Aliases: []string{"bk"},
		RunE:    fromBackupFunc,
	}

	importFromBrowserCmd = &cobra.Command{
		Use:   "browser",
		Short: "Import bookmarks from browser",
		RunE:  fromBrowserFunc,
	}

	importFromGitRepoCmd = &cobra.Command{
		Use:   "git",
		Short: "Import bookmarks from git repo",
		RunE:  cmdGit.GitImportCmd.RunE,
	}
)

func fromBrowserFunc(_ *cobra.Command, _ []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Browser(c, r); err != nil {
		return fmt.Errorf("import from browser: %w", err)
	}

	if handler.GitInitialized(config.App.Path.Git, r.Cfg.Fullpath()) {
		g, err := handler.NewGit(config.App.Path.Git)
		if err != nil {
			return err
		}
		g.Tracker.SetCurrent(g.NewRepo(config.App.DBPath))

		if err := handler.GitCommit(g, "Import from Browser"); err != nil {
			if errors.Is(err, git.ErrGitNothingToCommit) {
				return nil
			}
			return fmt.Errorf("commit: %w", err)
		}
	}

	return nil
}

func fromBackupFunc(commands *cobra.Command, args []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	bks, err := r.ListBackups()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if len(bks) == 0 {
		return db.ErrBackupNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	backupPath, err := handler.SelectBackupOne(c, bks)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcDB, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	c.T.SetInterruptFn(func(err error) {
		r.Close()
		srcDB.Close()
		sys.ErrAndExit(err)
	})

	if err := port.FromBackup(c, r, srcDB); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fromDatabaseFunc(_ *cobra.Command, _ []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	srcDB, err := handler.SelectDatabase(r.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Database(c, srcDB, r); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	gitPath := config.App.Path.Git
	if git.IsInitialized(gitPath) && git.IsTracked(gitPath, r.Cfg.Fullpath()) {
		g, err := handler.NewGit(gitPath)
		if err != nil {
			return err
		}
		gr := g.NewRepo(r.Cfg.Fullpath())
		g.Tracker.SetCurrent(gr)

		s := fmt.Sprintf("Import from [%s]", srcDB.Name())
		if err := handler.GitCommit(g, s); err != nil {
			if !errors.Is(err, git.ErrGitNothingToCommit) {
				return fmt.Errorf("commit: %w", err)
			}
		}
	}

	return nil
}
