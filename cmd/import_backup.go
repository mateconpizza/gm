package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// importBackupCmd imports bookmarks from a backup file.
var importBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "import bookmarks from backup",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		var bk *repo.SQLiteRepository
		if bk, err = importSelectBackup(r); err != nil {
			return fmt.Errorf("%w", err)
		}

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		if err := importFromDB(t, r, bk); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	// add cmd as a `import` subcommand
	importCmd.AddCommand(importBackupCmd)
}
