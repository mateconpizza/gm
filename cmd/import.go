package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

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
type importSourceNew struct {
	color color.ColorFn  // Color used of the `key`
	cmd   *cobra.Command // Cobra command
}

var mapSource = map[string]importSourceNew{
	"backups":  {color.BrightYellow, importBackupCmd},
	"browser":  {color.BrightGreen, importBrowserCmd},
	"database": {color.BrightBlue, importDatabaseCmd},
	"restore":  {color.BrightRed, importRestoreCmd},
}

// cleanDuplicateRecords removes duplicate bookmarks from the import process.
func cleanDuplicateRecords(r *Repo, bs *Slice) error {
	originalLen := bs.Len()
	bs.FilterInPlace(func(b *Bookmark) bool {
		return !r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
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
			menu.WithDefaultSettings(),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} info"),
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
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd(config.App.Cmd+" -n ./backup/"+srcDB.Cfg.Name+" {1}"),
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
			return db.Cfg.Name != destDB.Cfg.Name
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
			menu.WithDefaultSettings(),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n {1} info"),
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
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithHeader("select record/s to import", false),
			menu.WithPreview(),
			menu.WithPreviewCustomCmd("gm -n "+srcDB.Cfg.Name+" records {1}"),
			menu.WithInterruptFn(interruptFn),
		)

		return importFromDB(mb, t, destDB, srcDB)
	},
}

// importRestoreCmd imports/restore bookmarks from deleted table.
var importRestoreCmd = &cobra.Command{
	Use:     "restore",
	Aliases: []string{"deleted", "r"},
	Short:   "Import/restore bookmarks from deleted table",
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		terminal.ReadPipedInput(&args)
		// Switch tables and read from deleted table
		ts := r.Cfg.Tables
		r.SetMain(ts.Deleted)
		r.SetDeleted(ts.Main)
		// menu
		m := menu.New[Bookmark](
			menu.WithDefaultSettings(),
			menu.WithMultiSelection(),
			menu.WithPreviewCustomCmd("gm records {1}"),
			menu.WithHeader("select record/s to restore", false),
		)
		if Multiline {
			m.AddOpts(menu.WithMultilineView())
		}
		Menu = true
		bs, err := handleData(m, r, args)
		if err != nil {
			return err
		}
		if bs.Empty() {
			return repo.ErrRecordNoMatch
		}
		if Remove {
			return r.DeleteAndReorder(bs, r.Cfg.Tables.Main, r.Cfg.Tables.RecordsTagsDeleted)
		}

		return restoreDeleted(m, r, bs)
	},
}

// importFromDB imports bookmarks from the given database.
func importFromDB(m *menu.Menu[Bookmark], t *terminal.Term, destDB, srcDB *Repo) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	f.Header("Import from Database\n").Row("\n").Text(repo.RepoSummary(srcDB)).Row("\n").Flush()
	// prompt
	if !t.Confirm(f.Clear().Warning("continue?").String(), "y") {
		return handler.ErrActionAborted
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
	s := fmt.Sprintf("imported %d record/s\n", records.Len())
	t.ReplaceLine(1, f.Clear().Success(success+" "+s).Ln().String())

	return nil
}

// insertRecordsFromSource inserts records into the database.
func insertRecordsFromSource(t *terminal.Term, r *Repo, records *Slice) error {
	report := fmt.Sprintf("import %d records?", records.Len())
	f := frame.New(frame.WithColorBorder(color.BrightGray))
	if !t.Confirm(f.Row("\n").Header(report).String(), "y") {
		return handler.ErrActionAborted
	}
	sp := spinner.New(spinner.WithMesg(color.Yellow("importing record/s...").String()))
	sp.Start()
	if err := r.InsertMultiple(records); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp.Stop()
	success := color.BrightGreen("Successfully").Italic().String()
	msg := fmt.Sprintf(success+" imported %d record/s\n", records.Len())
	f.Clear().Success(msg).Flush()

	return nil
}

// handleRestore restores record/s from the deleted table.
func restoreDeleted(m *menu.Menu[Bookmark], r *Repo, bs *Slice) error {
	c := color.BrightYellow
	f := frame.New(frame.WithColorBorder(c))
	header := c("Restoring Bookmarks\n").String()
	f.Header(header).Ln().Flush()
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	}))

	prompt := color.BrightYellow("restore").Bold().String()
	if err := handler.Confirmation(m, t, bs, prompt, c); err != nil {
		return fmt.Errorf("%w", err)
	}
	sp := spinner.New(spinner.WithMesg(color.Yellow("restoring record/s...").String()))
	sp.Start()
	ts := r.Cfg.Tables
	if err := r.Restore(context.Background(), ts.Main, ts.Deleted, bs); err != nil {
		t.ClearLine(1)
		return fmt.Errorf("%w", err)
	}
	sp.Stop()
	f = frame.New(frame.WithColorBorder(color.Gray))
	success := color.BrightGreen("Successfully").Italic().String()
	t.ReplaceLine(1, f.Success(success+" bookmark/s restored\n").String())

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
		delimiter := format.BulletPoint
		keys := make([]string, 0, len(mapSource))
		for k, src := range mapSource {
			s := fmt.Sprintf("%-*s %s %s", maxSourceLen, src.color(k), delimiter, src.cmd.Short)
			keys = append(keys, s)
		}
		m := menu.New[string](
			menu.WithDefaultSettings(),
			menu.WithHeader("select a source to import from", false),
			menu.WithArgs("--no-bold"),
		)
		k, err := handler.Selection(m, keys, func(s *string) string { return *s })
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		selected := color.RemoveANSICodes(strings.Split(k[0], delimiter)[0])
		src, ok := mapSource[strings.TrimSpace(selected)]
		if !ok {
			return fmt.Errorf("%w: '%s'", ErrImportSourceNotFound, selected)
		}

		return src.cmd.RunE(cmd, args)
	},
}

func init() {
	rf := importRestoreCmd.Flags()
	rf.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	rf.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
	rf.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	rf.BoolVarP(&Multiline, "multiline", "M", false, "print data in formatted multiline (fzf)")
	rf.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	rf.StringSliceVarP(&Tags, "tags", "t", nil, "filter bookmarks by tag")
	importCmd.AddCommand(importBackupCmd, importBrowserCmd, importDatabaseCmd, importRestoreCmd)
	rootCmd.AddCommand(importCmd)
}
