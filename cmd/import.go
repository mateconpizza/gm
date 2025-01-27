package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
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
func cleanDuplicateRecords(r *repo.SQLiteRepository, bs *Slice) error {
	originalLen := bs.Len()
	bs.Filter(func(b Bookmark) bool {
		return !r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	})

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if originalLen != bs.Len() {
		skip := color.BrightYellow("skipping")
		s := fmt.Sprintf("%s %d duplicate bookmarks", skip, originalLen-bs.Len())
		f.Row().Ln().Warning(s).Ln().Render()
	}

	if bs.Empty() {
		return slice.ErrSliceEmpty
	}

	return nil
}

// importInsert inserts the bookmarks found from the import process.
func importInsert(r *repo.SQLiteRepository, bs *Slice) error {
	// FIX: merge with importInsertRecords
	bs.Filter(func(b Bookmark) bool {
		return !r.HasRecord(r.Cfg.Tables.Main, "url", b.URL)
	})
	if bs.Empty() {
		return nil
	}

	if err := r.InsertMultiple(bs); err != nil {
		return fmt.Errorf("%w", err)
	}

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	success := color.BrightGreen("Successfully").Italic().Bold()
	f.Row().Ln()
	f.Success(fmt.Sprintf("%s %d bookmarks imported.\n", success, bs.Len())).Render()

	return nil
}

// importInsertRecords inserts records into the database.
func importInsertRecords(t *terminal.Term, r *repo.SQLiteRepository, records *Slice) error {
	// FIX: merge with importInsert
	report := fmt.Sprintf("import %d records?", records.Len())
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	if !t.Confirm(f.Row().Ln().Header(report).String(), "y") {
		return handler.ErrActionAborted
	}

	sp := spinner.New(spinner.WithMesg(color.Yellow("importing record/s...").String()))
	sp.Start()
	defer sp.Stop()

	if err := r.InsertMultiple(records); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// importSelectSource prompts the user to select an import source.
func importSelectSource() (*importSource, error) {
	t := terminal.New(terminal.WithInterruptFn(func(err error) {
		sys.ErrAndExit(err)
	}))

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
	Use:   "import",
	Short: "import bookmarks from various sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		Menu = true
		source, err := importSelectSource()
		if err != nil {
			return err
		}

		return source.cmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
