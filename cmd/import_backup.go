package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// selectBackup prompts the user to select a backup file.
func selectBackup(r *repo.SQLiteRepository) (*repo.SQLiteRepository, error) {
	files, err := repo.Backups(r)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	backups := slice.New[repo.SQLiteRepository]()
	files.ForEach(func(f string) {
		c := repo.NewSQLiteCfg(filepath.Dir(f))
		c.SetName(filepath.Base(f))
		bk, err := repo.New(c)
		if err != nil {
			return
		}
		backups.Append(bk)
	})

	if backups.Len() == 1 {
		bk := backups.Item(0)
		return &bk, nil
	}

	m := menu.New[repo.SQLiteRepository](
		menu.WithDefaultSettings(),
		menu.WithHeader(fmt.Sprintf("choose a backup from '%s'", r.Cfg.Name), false),
		menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} -i"),
	)

	fmtter := func(r *repo.SQLiteRepository) string {
		main := fmt.Sprintf("(main: %d, ", repo.CountRecords(r, r.Cfg.Tables.Main))
		deleted := fmt.Sprintf("deleted: %d)", repo.CountRecords(r, r.Cfg.Tables.Deleted))
		records := color.Gray(main + deleted).Italic()

		return filepath.Base(r.Cfg.Fullpath()) + " " + records.String()
	}

	backupSlice, err := handler.Selection(m, backups.Items(), fmtter)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return &backupSlice[0], nil
}

// importBackupCmd imports bookmarks from a backup file.
var importBackupCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"bk", "backups", "backup"},
	Short:   "import bookmarks from backup",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		var bk *repo.SQLiteRepository
		if bk, err = selectBackup(r); err != nil {
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
