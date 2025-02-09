package cmd

import (
	"errors"
	"fmt"
	"log"

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
	"github.com/haaag/gm/internal/sys/spinner"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrImportSameDatabase   = errors.New("cannot import into the same database")
)

// importSource defines a bookmark import source.
type importSource struct {
	key   string         // Shortname of the source
	name  string         // Name of the source
	color color.ColorFn  // Color used of the `key`
	cmd   *cobra.Command // Cobra command
}

// registeredImportSources contains the registered import sources.
var registeredImportSources = []importSource{
	{"a", "database", color.BrightBlue, importDatabaseCmd},
	{"s", "browser", color.BrightGreen, importBrowserCmd},
	{"d", "restore", color.BrightRed, importRestoreCmd},
	{"w", "backup", color.BrightOrange, importBackupCmd},
}

// getSource returns the import source for the given key.
func getSource(key string) (*importSource, bool) {
	for _, s := range registeredImportSources {
		if s.key == key {
			return &s, true
		}
	}
	log.Printf("import source not found: '%s'", key)

	return nil, false
}

// cleanDuplicateRecords removes duplicate bookmarks from the import process.
func cleanDuplicateRecords(r *Repo, bs *Slice) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *Bookmark) bool {
		return !r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	})
	if originalLen != bs.Len() {
		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Render()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// selectBackup prompts the user to select a backup file.
func selectBackup(m *menu.Menu[Repo], r *Repo) (*slice.Slice[Repo], error) {
	backups, err := repo.Databases(r.Cfg.Backup.Path)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if backups.Len() == 1 {
		return backups, nil
	}
	if backups.Len() == 0 {
		return nil, repo.ErrBackupNotFound
	}
	backupSlice, err := handler.Selection(m, *backups.Items(), repo.SummaryBackupLine)
	if err != nil {
		return backups, fmt.Errorf("%w", err)
	}
	backups.Set(&backupSlice)

	return backups, nil
}

// importBackupCmd imports bookmarks from a backup file.
var importBackupCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"b", "bk", "backups"},
	Short:   "import bookmarks from backup",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		m := menu.New[Repo](
			menu.WithDefaultSettings(),
			menu.WithHeader("choose a backup to import from", false),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		backups, err := selectBackup(m, r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		fromBK := backups.Item(0)
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		var toDB *Repo
		if DBName != config.DefaultDBName {
			toDB = r
		} else {
			toDB, err = selectRepo("choose a database to import to")
			if err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		mb := menu.New[Bookmark](
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithHeader("select record/s to import", false),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd(config.App.Cmd+" -n ./backup/"+fromBK.Cfg.Name+" {1}"),
		)

		return importFromDB(mb, t, toDB, &fromBK)
	},
}

// importBrowserCmd imports bookmarks from a browser.
var importBrowserCmd = &cobra.Command{
	Use:     "browser",
	Aliases: []string{"b"},
	Short:   "import bookmarks from browser",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		return importFromBrowser(t, r)
	},
}

// importDatabaseCmd imports bookmarks from a database.
var importDatabaseCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"d", "db"},
	Short:   "import bookmarks from database",
	RunE: func(_ *cobra.Command, _ []string) error {
		toDB, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer toDB.Close()
		interruptFn := func(err error) {
			toDB.Close()
			sys.ErrAndExit(err)
		}
		dbs, err := repo.Databases(Cfg.Path)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer dbs.ForEachMut(func(r *repo.SQLiteRepository) { r.Close() })
		dbs.FilterInPlace(func(db *Repo) bool {
			return db.Cfg.Name != toDB.Cfg.Name
		})
		if dbs.Len() == 0 {
			return repo.ErrDBsNotFound
		}
		ppp := config.App.Cmd + " db -n {1} info"
		fmt.Printf("ppp: %v\n", ppp)
		t := terminal.New(terminal.WithInterruptFn(interruptFn))
		m := menu.New[Repo](
			menu.WithInterruptFn(interruptFn),
			menu.WithDefaultSettings(),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n {1} info"),
			menu.WithHeader("choose a database to import from", false),
		)
		item, err := handler.Selection(m, *dbs.Items(), repo.RepoSummaryRecords)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		fromDB := &item[0]
		defer fromDB.Close()
		mb := menu.New[Bookmark](
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithHeader("select record/s to import", false),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd("gm -n "+fromDB.Cfg.Name+" records {1}"),
		)

		return importFromDB(mb, t, toDB, fromDB)
	},
}

// importFromDB imports bookmarks from the given database.
func importFromDB(m *menu.Menu[Bookmark], t *terminal.Term, toDB, fromDB *Repo) error {
	// set interrupt handler
	interruptFn := func(err error) {
		toDB.Close()
		fromDB.Close()
		log.Println("importFromDB interrupted")
		sys.ErrAndExit(err)
	}
	t.SetInterruptFn(interruptFn)
	m.SetInterruptFn(interruptFn)
	defer t.CancelInterruptHandler()
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header("Import from Database\n").Row("\n").Text(repo.RepoSummary(fromDB)).Row("\n").Render()
	// prompt
	if !t.Confirm(f.Clean().Warning("continue?").String(), "y") {
		return handler.ErrActionAborted
	}
	t.ClearLine(1)
	Menu = true
	records, err := handleData(m, fromDB, []string{})
	if err != nil {
		return err
	}
	t.ClearLine(1)
	if err := cleanDuplicateRecords(toDB, records); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Clean().Row("\n").Mid("no new bookmark found, skipping import\n").Render()
			return nil
		}

		return err
	}
	if err := insertRecordsFromSource(t, toDB, records); err != nil {
		return err
	}
	// remove prompt
	success := color.BrightGreen("Successfully").Italic().Bold().String()
	s := fmt.Sprintf("imported %d record/s", records.Len())
	t.ReplaceLine(1, f.Clean().Success(success+" "+s).Ln().String())

	return nil
}

// insertRecordsFromSource inserts records into the database.
func insertRecordsFromSource(t *terminal.Term, r *Repo, records *Slice) error {
	report := fmt.Sprintf("import %d records?", records.Len())
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if !t.Confirm(f.Row().Ln().Header(report).String(), "y") {
		return handler.ErrActionAborted
	}
	sp := spinner.New(spinner.WithMesg(color.Yellow("importing record/s...").String()))
	sp.Start()
	if err := r.InsertMultiple(records); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp.Stop()
	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s", records.Len())
	f.Clean().Success(msg).Ln().Render()

	return nil
}

// selectSource prompts the user to select an import source.
func selectSource() (*importSource, error) {
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		sys.ErrAndExit(err)
	}))
	defer t.CancelInterruptHandler()

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header("Supported Sources").Ln().Row().Ln()
	for _, src := range registeredImportSources {
		s := src.color(src.key).Bold().String() + " " + src.cmd.Short
		f.Mid(s).Ln()
	}

	lines := format.CountLines(f.String())
	f.Render().Clean()
	f.Row().Ln().Footer("import from which source?").Render()
	name := t.Prompt(" ")

	t.ClearLine(lines + 1)
	source, found := getSource(name)
	if !found {
		return nil, fmt.Errorf("%w: '%s'", ErrImportSourceNotFound, name)
	}
	log.Printf("source: '%s' called", source.name)

	return source, nil
}

// importCmd imports bookmarks from various sources.
var importCmd = &cobra.Command{
	Use:     "import",
	Aliases: []string{"i"},
	Short:   "import bookmarks from various sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		// enable menu
		Menu = true
		src, err := selectSource()
		if err != nil {
			return err
		}

		return src.cmd.RunE(cmd, args)
	},
}

func init() {
	importCmd.AddCommand(importBackupCmd, importBrowserCmd, importDatabaseCmd)
	rootCmd.AddCommand(importCmd)
}
