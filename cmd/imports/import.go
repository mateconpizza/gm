package imports

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	cmdGit "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/repository"
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
		PersistentPostRunE: gitUpdate,
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
		Short: cmdGit.GitImportCmd.Short,
		RunE:  cmdGit.GitImportCmd.RunE,
	}
)

func fromBrowserFunc(_ *cobra.Command, _ []string) error {
	store, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	r := repository.New(store)
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

func fromBackupFunc(command *cobra.Command, args []string) error {
	destStore, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	destRepo := repository.New(destStore)
	defer destRepo.Close()

	dbName := files.StripSuffixes(destStore.Name())
	bks, err := files.List(config.App.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(bks) == 0 {
		return db.ErrBackupNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			destRepo.Close()
			sys.ErrAndExit(err)
		}))),
	)

	backupPath, err := handler.SelectBackupOne(c, bks)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcStore, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	srcRepo := repository.New(srcStore)
	defer srcRepo.Close()

	c.T.SetInterruptFn(func(err error) {
		destRepo.Close()
		srcRepo.Close()
		sys.ErrAndExit(err)
	})

	if err := port.FromBackup(c, destRepo, srcRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fromDatabaseFunc(command *cobra.Command, _ []string) error {
	store, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	r := repository.New(store)
	defer r.Close()

	srcDB, err := handler.SelectDatabase(r.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	rSrc := repository.New(srcDB)
	defer rSrc.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			rSrc.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Database(c, rSrc, r); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	return nil
}

func gitUpdate(command *cobra.Command, _ []string) error {
	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() {
		return nil
	}

	if err := gr.Export(); err != nil {
		return err
	}

	return gr.Commit(command.Short)
}
