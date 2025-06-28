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
		Use:     "imp",
		Aliases: []string{"i", "import"},
		Short:   "Import from various sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	importFromDatabaseCmd = &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Import from database",
		RunE:    fromDatabaseFunc,
	}

	importFromBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   "Import from backup",
		Aliases: []string{"bk"},
		RunE:    fromBackupFunc,
	}

	importFromBrowserCmd = &cobra.Command{
		Use:   "browser",
		Short: "Import from browser",
		RunE:  fromBrowserFunc,
	}

	importFromGitRepoCmd = &cobra.Command{
		Use:   "git",
		Short: "Import from git URL",
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

	return nil
}
