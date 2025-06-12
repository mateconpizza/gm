package handler

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/db"
	"github.com/mateconpizza/gm/internal/format/color"
	"github.com/mateconpizza/gm/internal/format/frame"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui/menu"
)

// ImportFromDB imports bookmarks from the given database.
func ImportFromDB(
	cmd *cobra.Command,
	m *menu.Menu[bookmark.Bookmark],
	t *terminal.Term,
	destDB, srcDB *db.SQLiteRepository,
) error {
	// FIX: move to `importer` package
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	i := color.BrightMagenta("Import").Bold().String() + " from Database\n"
	f.Header(i).Row("\n").Text(db.RepoSummary(srcDB)).Row("\n").Flush()
	// prompt
	if err := t.ConfirmErr(f.Clear().Question("continue?").String(), "y"); err != nil {
		return fmt.Errorf("%w", err)
	}
	t.ClearLine(1)
	// Get command 'records'
	recordsCmd, _, err := cmd.Root().Find([]string{"records"})
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	err = recordsCmd.Flags().Set("menu", "true")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	records, err := Data(recordsCmd, m, srcDB, []string{})
	if err != nil {
		return err
	}
	t.ClearLine(1)
	if err := cleanDuplicates(destDB, records); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Clear().Row("\n").Mid("no new bookmark found, skipping import\n").Flush()
			return nil
		}

		return err
	}

	if err := port.IntoRepo(t, destDB, records); err != nil {
		return fmt.Errorf("inserting records: %w", err)
	}

	return GitCommit("Import")
}

func ImportFromBackupNew(destDB *db.SQLiteRepository) error {
	backupPath, err := SelectRepoBackup(destDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcDB, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		destDB.Close()
		srcDB.Close()
		sys.ErrAndExit(err)
	}))

	f := frame.New(frame.WithColorBorder(color.Gray))

	if err := port.FromBackup(t, f, destDB, srcDB); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// ImportFromBackup imports bookmarks from a backup.
func ImportFromBackup(cmd *cobra.Command, args []string) error {
	destDB, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destDB.Close()

	backupPath, err := SelectRepoBackup(destDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcDB, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	interruptFn := func(err error) {
		destDB.Close()
		srcDB.Close()
		sys.ErrAndExit(err)
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithMultiSelection(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithPreview(fmt.Sprintf("%s -n ./backup/%s {1}", config.App.Cmd, srcDB.Name())),
		menu.WithInterruptFn(interruptFn),
		menu.WithHeader("select record/s to import from '"+srcDB.Name()+"'", false),
	)

	t := terminal.New(terminal.WithInterruptFn(interruptFn))
	defer t.CancelInterruptHandler()

	return ImportFromDB(cmd, m, t, destDB, srcDB)
}

// ImportFromDatabase imports bookmarks from a database.
func ImportFromDatabase() error {
	srcDB, err := SelectDatabase()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	if err := port.Database(srcDB); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	if err := GitCommit("Import from Browser"); err != nil {
		if errors.Is(err, git.ErrGitNothingToCommit) {
			return nil
		}
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}
