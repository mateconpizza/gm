package imports

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	cmdGit "github.com/mateconpizza/gm/cmd/git"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var (
	ErrImportSourceNotFound = errors.New("import source not found")
	ErrMissingArg           = errors.New("missing argument")
)

func init() {
	importFromCmd.AddCommand(
		importFromBackupCmd,
		importFromBrowserCmd,
		importFromDatabaseCmd,
		importFromGitRepoCmd,
		importFromHTML,
	)
	cmd.Root.AddCommand(importFromCmd)
}

// imports bookmarks from various sources.
var (
	importFromCmd = &cobra.Command{
		Use:     "imp",
		Aliases: []string{"i", "import"},
		Short:   "Import from various sources",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
		PersistentPostRunE: gitUpdate,
	}

	importFromDatabaseCmd = &cobra.Command{
		Use:     "database",
		Aliases: []string{"db"},
		Short:   "Import from database",
		RunE:    fromDatabaseFunc,
	}

	importFromBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   "Import from backup",
		Aliases: []string{"bk"},
		RunE:    fromBackupFunc,
	}

	importFromBrowserCmd = &cobra.Command{
		Use:   "browser",
		Short: "Import from browser",
		RunE:  fromBrowserFunc,
	}

	importFromGitRepoCmd = &cobra.Command{
		Use:   "git",
		Short: cmdGit.GitImportCmd.Short,
		RunE:  cmdGit.GitImportCmd.RunE,
	}

	importFromHTML = &cobra.Command{
		Use:   "html",
		Short: "Import from HTML Netscape file",
		RunE:  fromHTMLNetscapeFunc,
	}
)

func fromBrowserFunc(_ *cobra.Command, _ []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Browser(c, r); err != nil {
		return fmt.Errorf("import from browser: %w", err)
	}

	return nil
}

func fromBackupFunc(command *cobra.Command, args []string) error {
	destRepo, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer destRepo.Close()

	dbName := files.StripSuffixes(destRepo.Name())
	bks, err := files.List(config.App.Path.Backup, "*_"+dbName+".db*")
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	if len(bks) == 0 {
		return db.ErrBackupNotFound
	}

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			destRepo.Close()
			sys.ErrAndExit(err)
		}))),
	)

	backupPath, err := handler.SelectBackupOne(c, bks)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	srcRepo, err := db.New(backupPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer srcRepo.Close()

	c.T.SetInterruptFn(func(err error) {
		destRepo.Close()
		srcRepo.Close()
		sys.ErrAndExit(err)
	})

	if err := port.FromBackup(c, destRepo, srcRepo); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func fromDatabaseFunc(command *cobra.Command, _ []string) error {
	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	// FIX: refactor `SelectDatabase`, return a string (fullpath)
	srcDB, err := handler.SelectDatabase(r.Cfg.Fullpath())
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	rSrc, err := db.New(srcDB)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer rSrc.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			rSrc.Close()
			sys.ErrAndExit(err)
		}))),
	)

	if err := port.Database(c, rSrc, r); err != nil {
		return fmt.Errorf("import from database: %w", err)
	}

	return nil
}

//nolint:funlen //ignore
func fromHTMLNetscapeFunc(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w: %q", ErrMissingArg, "filename")
	}

	file, err := os.Open(args[0])
	if err != nil {
		log.Printf("Error opening file: %v, %q\n", err, args[0])
		return err
	}
	defer func() { _ = file.Close() }()

	if err := bookio.IsValidNetscapeFile(file); err != nil {
		return err
	}

	bp := bookio.NewHTMLParser()
	nbs, err := bp.ParseHTML(file)
	if err != nil {
		return err
	}

	bs := make([]*bookmark.Bookmark, 0, len(nbs))
	for i := range nbs {
		bs = append(bs, bookio.FromNetscape(&nbs[i]))
	}

	r, err := db.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)
	c.F.Success(fmt.Sprintf("Found %d bookmarks from %q\n", len(nbs), file.Name())).Flush()

	deduplicated := port.Deduplicate(c, r, bs)
	n := len(deduplicated)
	if n == 0 {
		return bookmark.ErrBookmarkNotFound
	}

	opt, err := c.Choose(fmt.Sprintf("Import %d bookmarks?", n), []string{"yes", "no", "select"}, "y")
	if err != nil {
		return err
	}

	switch strings.ToLower(opt) {
	case "n", "no":
		return sys.ErrActionAborted
	case "s", "select":
		m := menu.New[*bookmark.Bookmark](
			menu.WithInterruptFn(c.T.InterruptFn),
			menu.WithMultiSelection(),
			menu.WithUseDefaults(),
			menu.WithHeader("select record/s to import", false),
		)

		m.SetItems(deduplicated)
		m.SetPreprocessor(func(b **bookmark.Bookmark) string { return txt.Oneline(*b) })
		deduplicated, err = m.Select()
		if err != nil {
			return err
		}
		n = len(deduplicated)
	case "y", "yes":
		fmt.Println("importing items")
	}

	if err := r.InsertMany(context.Background(), deduplicated); err != nil {
		return err
	}
	fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d bookmarks", n)))

	return nil
}

func gitUpdate(command *cobra.Command, _ []string) error {
	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return err
	}
	if !gr.IsTracked() {
		return nil
	}

	if err := gr.Export(); err != nil {
		return err
	}

	return gr.Commit(command.Short)
}
