package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/terminal"
)

// importDatabaseCmd imports bookmarks from a database.
var importDatabaseCmd = &cobra.Command{
	Use:     "database",
	Short:   "import bookmarks from database",
	Aliases: []string{"db"},
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

		fromDB, err := importSelectDatabase(r)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return importFromDB(t, r, fromDB)
	},
}

// importSelectDatabase prompts the user to select a database.
func importSelectDatabase(r *repo.SQLiteRepository) (*repo.SQLiteRepository, error) {
	db, err := handler.ChooseDB(r)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	defer db.Close()

	return db, nil
}

// importFromDB imports bookmarks from the given database.
func importFromDB(t *terminal.Term, toDB, fromDB *repo.SQLiteRepository) error {
	// set interrupt handler
	t.SetInterruptFn(func(err error) {
		toDB.Close()
		fromDB.Close()
		log.Println("importFromDB interrupted")
		sys.ErrAndExit(err)
	})

	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	f.Header("Import from Database").Ln().
		Row().Ln().Text(repo.Summary(fromDB)).
		Row().Ln().Render()

	f.Clean().Warning("continue?")

	if !t.Confirm(f.String(), "y") {
		return handler.ErrActionAborted
	}
	t.ClearLine(1)

	Menu = true
	m := menu.New[Bookmark](
		menu.WithDefaultSettings(),
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s to import", false),
		menu.WithPreviewCustomCmd(config.App.Cmd+" -n "+fromDB.Cfg.Name+" {1}"),
	)
	records, err := handleData(m, fromDB, []string{})
	if err != nil {
		return err
	}

	t.ClearLine(1)
	if err := cleanDuplicateRecords(toDB, records); err != nil {
		if errors.Is(err, slice.ErrSliceEmpty) {
			f.Clean().Row().Ln().Mid("no new bookmark found, skipping import").Ln().Render()
			return nil
		}

		return err
	}

	if err := importInsertRecords(t, toDB, records); err != nil {
		return err
	}

	// remove prompt
	t.ClearLine(1)
	success := color.BrightGreen("Successfully").Italic().Bold().String()
	s := fmt.Sprintf("imported %d record/s", records.Len())
	f.Clean().Success(success + " " + s).Ln().Render()

	return nil
}

func init() {
	// add cmd as a `import` subcommand
	importCmd.AddCommand(importDatabaseCmd)
}
