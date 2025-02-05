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

var ErrImportSourceNotFound = errors.New("import source not found")

// importSource defines a bookmark import source.
type importSource struct {
	key   string
	name  string
	color color.ColorFn
	cmd   *cobra.Command
}

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
	backups, err := repo.Backups(r)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	if backups.Len() == 1 {
		bk := backups.Item(0)
		backups.Set(&[]Repo{bk})
		return backups, nil
	}
	backupSlice, err := handler.Selection(m, backups.Items(), repo.SummaryBackupLine)
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
			menu.WithHeader(fmt.Sprintf("choose a backup from '%s'", r.Cfg.Name), false),
			menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} info"),
		)
		backups, err := selectBackup(m, r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		bk := backups.Item(0)

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		if err := importFromDB(t, r, &bk); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
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
	msg := fmt.Sprintf("%s imported %d record/s", success, records.Len())
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
	importCmd.AddCommand(importBackupCmd)
	importCmd.AddCommand(importBrowserCmd)
	importCmd.AddCommand(importDatabaseCmd)
	rootCmd.AddCommand(importCmd)
}
