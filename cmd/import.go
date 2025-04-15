package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrImportSameDatabase   = errors.New("cannot import into the same database")
)

// importSource defines a bookmark import source.
type importSourceNew struct {
	color color.ColorFn  // Color used of the `key`
	cmd   *cobra.Command // Cobra command
}

var mapSource = map[string]importSourceNew{
	"backups":  {color.BrightYellow, importBackupCmd},
	"browser":  {color.BrightGreen, importBrowserCmd},
	"database": {color.BrightBlue, importDatabaseCmd},
}

// cleanDuplicateRecords removes duplicate bookmarks from the import process.
func cleanDuplicateRecords(r *Repo, bs *Slice) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *Bookmark) bool {
		_, exists := r.Has(b.URL)
		return !exists
	})
	if originalLen != bs.Len() {
		f := frame.New(frame.WithColorBorder(color.BrightGray))
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Flush()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// importBackupCmd imports bookmarks from a backup file.
var importBackupCmd = &cobra.Command{
	Use:     "backup",
	Short:   "Import bookmarks from backup",
	Aliases: []string{"b", "bk", "backups"},
	RunE: func(_ *cobra.Command, _ []string) error {
		destDB, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer destDB.Close()
		mPaths := menu.New[string](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithPreview(config.App.Cmd+" db -n ./backup/{1} info"),
			menu.WithHeader("choose a backup to import from", false),
		)
		selected, err := handler.Selection(mPaths, destDB.Cfg.Backup.Files, func(p *string) string {
			return filepath.Base(*p)
		})
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		srcDB, err := repo.New(repo.NewSQLiteCfg(selected[0]))
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer srcDB.Close()
		interruptFn := func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}
		t := terminal.New(terminal.WithInterruptFn(interruptFn))
		defer t.CancelInterruptHandler()
		mb := menu.New[Bookmark](
			menu.WithUseDefaults(),
			menu.WithMultiSelection(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithPreview(config.App.Cmd+" -n ./backup/"+srcDB.Name()+" {1}"),
			menu.WithInterruptFn(interruptFn),
			menu.WithHeader("select record/s to import", false),
		)

		return importFromDB(mb, t, destDB, srcDB)
	},
}

// importBrowserCmd imports bookmarks from a browser.
var importBrowserCmd = &cobra.Command{
	Use:     "browser",
	Aliases: []string{"b"},
	Short:   "Import bookmarks from browser",
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
	Short:   "Import bookmarks from database",
	Long:    "/",
	RunE: func(_ *cobra.Command, _ []string) error {
		destDB, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer destDB.Close()
		dbs, err := repo.Databases(Cfg.Path)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer dbs.ForEachMut(func(r *repo.SQLiteRepository) { r.Close() })
		dbs.FilterInPlace(func(db *Repo) bool {
			return db.Name() != destDB.Name()
		})
		if dbs.Len() == 0 {
			return repo.ErrDBsNotFound
		}
		interruptFn := func(err error) {
			destDB.Close()
			sys.ErrAndExit(err)
		}
		m := menu.New[Repo](
			menu.WithInterruptFn(interruptFn),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithUseDefaults(),
			menu.WithPreview(config.App.Cmd+" db -n {1} info"),
			menu.WithHeader("choose a database to import from", false),
		)
		item, err := handler.Selection(m, *dbs.Items(), repo.RepoSummaryRecords)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		srcDB := &item[0]
		defer srcDB.Close()
		interruptFn = func(err error) {
			destDB.Close()
			srcDB.Close()
			sys.ErrAndExit(err)
		}
		t := terminal.New(terminal.WithInterruptFn(interruptFn))
		defer t.CancelInterruptHandler()
		mb := menu.New[Bookmark](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithMultiSelection(),
			menu.WithHeader("select record/s to import", false),
			menu.WithPreview(config.App.Cmd+" -n "+srcDB.Name()+" records {1}"),
			menu.WithInterruptFn(interruptFn),
		)

		return importFromDB(mb, t, destDB, srcDB)
	},
}

// importFromDB imports bookmarks from the given database.
func importFromDB(m *menu.Menu[Bookmark], t *terminal.Term, destDB, srcDB *Repo) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	i := color.BrightMagenta("Import").Bold().String() + " from Database\n"
	f.Header(i).Row("\n").Text(repo.RepoSummary(srcDB)).Row("\n").Flush()
	// prompt
	if !t.Confirm(f.Clear().Question("continue?").String(), "y") {
		return sys.ErrActionAborted
	}
	t.ClearLine(1)
	Menu = true
	records, err := handleData(m, srcDB, []string{})
	if err != nil {
		return err
	}
	t.ClearLine(1)
	if err := cleanDuplicateRecords(destDB, records); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Clear().Row("\n").Mid("no new bookmark found, skipping import\n").Flush()
			return nil
		}

		return err
	}
	if err := insertRecordsFromSource(t, destDB, records); err != nil {
		return err
	}
	// remove prompt
	success := color.BrightGreen("Successfully").Italic().Bold().String()
	s := fmt.Sprintf("imported %d record/s", records.Len())
	t.ReplaceLine(2, f.Clear().Success(success+" "+s).String())

	return nil
}

// insertRecordsFromSource inserts records into the database.
func insertRecordsFromSource(t *terminal.Term, r *Repo, records *Slice) error {
	report := fmt.Sprintf("import %d records?", records.Len())
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if !t.Confirm(f.Row("\n").Header(report).String(), "y") {
		return sys.ErrActionAborted
	}
	sp := rotato.New(
		rotato.WithMesg("importing record/s..."),
		rotato.WithMesgColor(rotato.ColorYellow),
	)
	sp.Start()
	if err := r.InsertMany(context.Background(), records); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp.Done()
	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s\n", records.Len())
	f.Clear().Success(msg).Flush()

	return nil
}

// importCmd imports bookmarks from various sources.
var importCmd = &cobra.Command{
	Use:     "import",
	Aliases: []string{"i"},
	Short:   "Import bookmarks from various sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		Menu = true
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			sys.ErrAndExit(err)
		}))
		defer t.CancelInterruptHandler()

		maxSourceLen := 0
		for k, src := range mapSource {
			maxSourceLen = max(maxSourceLen, len(src.color(k).String()))
		}
		delimiter := format.NBSP
		keys := make([]string, 0, len(mapSource))
		for k, src := range mapSource {
			s := fmt.Sprintf("%-*s %s%s", maxSourceLen, src.color(k), delimiter, src.cmd.Short)
			keys = append(keys, s)
		}
		m := menu.New[string](
			menu.WithUseDefaults(),
			menu.WithSettings(config.Fzf.Settings),
			menu.WithHeader("select a source to import from", false),
			menu.WithArgs("--no-bold"),
		)
		k, err := handler.Selection(m, keys, func(s *string) string { return *s })
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		selected := menu.ANSICodeRemover(strings.Split(k[0], delimiter)[0])
		src, ok := mapSource[strings.TrimSpace(selected)]
		if !ok {
			return fmt.Errorf("%w: %q", ErrImportSourceNotFound, selected)
		}

		return src.cmd.RunE(cmd, args)
	},
}

func init() {
	importCmd.AddCommand(importBackupCmd, importBrowserCmd, importDatabaseCmd)
	rootCmd.AddCommand(importCmd)
}
