package handler

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/browser"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// ImportFromDB imports bookmarks from the given database.
func ImportFromDB(
	cmd *cobra.Command,
	m *menu.Menu[bookmark.Bookmark],
	t *terminal.Term,
	destDB, srcDB *repo.SQLiteRepository,
) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	i := color.BrightMagenta("Import").Bold().String() + " from Database\n"
	f.Header(i).Row("\n").Text(repo.RepoSummary(srcDB)).Row("\n").Flush()
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
	if err := insertRecordsToRepo(t, destDB, records); err != nil {
		return err
	}
	// remove prompt
	success := color.BrightGreen("Successfully").Italic().Bold().String()
	s := fmt.Sprintf("imported %d record/s", records.Len())
	t.ReplaceLine(2, f.Clear().Success(success+" "+s).String())

	return nil
}

// ImportFromBrowser imports bookmarks from the given browser.
func ImportFromBrowser(cmd *cobra.Command, args []string) error {
	r, err := repo.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	}))
	br, ok := getBrowser(selectBrowser(t))
	if !ok {
		return fmt.Errorf("%w", browser.ErrBrowserUnsupported)
	}
	if err := br.LoadPaths(); err != nil {
		return fmt.Errorf("%w", err)
	}
	// find bookmarks
	bs, err := br.Import(t, config.App.Force)
	if err != nil {
		return fmt.Errorf("browser %q: %w", br.Name(), err)
	}
	// clean and process found bookmarks
	if err := parseFoundFromBrowser(t, r, bs); err != nil {
		return err
	}
	if bs.Len() == 0 {
		return nil
	}

	return insertRecordsToRepo(t, r, bs)
}

// ImportFromBackup imports bookmarks from a backup.
func ImportFromBackup(cmd *cobra.Command, args []string) error {
	destDB, err := repo.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destDB.Close()

	backupPath, err := SelectRepoBackup(destDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcDB, err := repo.New(backupPath)
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

// ImportFromDatabase is the RunE for “databaseold” (import bookmarks).
func ImportFromDatabase(cmd *cobra.Command, _ []string) error {
	// build list of candidate .db files
	dbs := slice.New[string]()
	fs, err := files.FindByExtList(config.App.Path.Data, ".db")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	dbs.Set(&fs)
	dbs = dbs.Filter(func(r string) bool {
		return filepath.Base(r) != config.App.DBName
	})
	// ask the user which one to import from
	s, err := SelectItemFrom(*dbs.Items(), "choose a database to import from")
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if !files.Exists(s) {
		return fmt.Errorf("%w: %q", repo.ErrDBNotFound, s)
	}
	// open source and destination
	srcDB, err := repo.New(s)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcDB.Close()

	destPath := filepath.Join(config.App.Path.Data, config.App.DBName)
	destDB, err := repo.New(destPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destDB.Close()

	// build interrupt cleanup
	interruptFn := func(err error) {
		destDB.Close()
		srcDB.Close()
		sys.ErrAndExit(err)
	}

	m := menu.New[bookmark.Bookmark](
		menu.WithUseDefaults(),
		menu.WithSettings(config.Fzf.Settings),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
		menu.WithPreview(config.App.Cmd+" -n "+srcDB.Name()+" records {1}"),
		menu.WithInterruptFn(interruptFn),
	)
	t := terminal.New(terminal.WithInterruptFn(interruptFn))

	return ImportFromDB(cmd, m, t, destDB, srcDB)
}
