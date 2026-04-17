package in

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/bookmark/port"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/files"
)

var ErrMissingArg = errors.New("missing argument")

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "import",
		Aliases: []string{"imp", "i"},
		Short:   "import bookmarks",
		RunE:    cli.HookHelp,
	}

	c.AddCommand(
		newHTMLCmd(cfg),
		newBrowserCmd(cfg),
		newFromDatabaseCmd(cfg),
		newFromBackupCmd(cfg),
	)

	cmdutil.HideFlag(c, "help")

	return c
}

func newFromDatabaseCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "database",
		Short:   "import from database",
		Aliases: []string{"db"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rDest, err := db.New(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			defer rDest.Close()

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			// FIX: refactor `SelectDatabase`, return a string (fullpath)
			srcDB, err := handler.SelectDatabase(d, rDest.Cfg.Fullpath())
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			rSrc, err := db.New(srcDB)
			if err != nil {
				return err
			}
			defer rSrc.Close()

			return port.Database(d, rSrc, rDest)
		},
	}

	return c
}

func newFromBackupCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "backup",
		Short:   "import from backup",
		Aliases: []string{"bk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			destRepo, err := db.New(cfg.DBPath)
			if err != nil {
				return err
			}
			defer destRepo.Close()

			dbName := files.StripSuffixes(destRepo.Name())
			bks, err := files.List(cfg.Path.Backup, "*_"+dbName+".db*")
			if err != nil {
				return err
			}

			if len(bks) == 0 {
				return db.ErrBackupNotFound
			}

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			backupPath, err := handler.SelectBackupOne(d, bks)
			if err != nil {
				return err
			}

			srcRepo, err := db.New(backupPath)
			if err != nil {
				return err
			}
			defer srcRepo.Close()

			return port.FromBackup(d, destRepo, srcRepo)
		},
	}

	return c
}

func newBrowserCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "browser",
		Short: "import from browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := db.New(cfg.DBPath)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			defer r.Close()

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			return port.Browser(d)
		},
	}

	return c
}

func newHTMLCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "html",
		Short: "import from HTML Netscape file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Flags.Path == "" {
				return fmt.Errorf("%w: %q", ErrMissingArg, "filename")
			}

			file, err := os.Open(cfg.Flags.Path)
			if err != nil {
				log.Printf("Error opening file: %v, %q\n", err, cfg.Flags.Path)
				return err
			}
			defer func() {
				if err := file.Close(); err != nil {
					slog.Error("Err closing file", "file", cfg.Flags.Path)
				}
			}()

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

			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			c := d.Console()
			s := fmt.Sprintf("Found %d bookmarks from %q\n", len(nbs), file.Name())
			c.Frame().Success(c.Palette().Italic.Sprint(s)).Flush()

			deduplicated := port.Deduplicate(cmd.Context(), c, d.DB, bs)
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
					menu.WithOutputColor(cfg.Flags.Color),
					menu.WithHeader("select record/s to import"),
					menu.WithInterruptFn(c.Term().InterruptFn),
					menu.WithMultiSelection(),
				)

				m.SetFormatter(func(b **bookmark.Bookmark) string { return txt.Oneline(c, *b) })
				deduplicated, err = m.Select(deduplicated)
				if err != nil {
					return err
				}
				n = len(deduplicated)
			case "y", "yes":
				// FIX: finish implementation
				fmt.Println("importing items...well, not implemented yet.")
			}

			if err := d.DB.InsertMany(cmd.Context(), deduplicated); err != nil {
				return err
			}
			fmt.Println(c.SuccessMesg(fmt.Sprintf("imported %d bookmarks", n)))

			return nil
		},
	}

	c.Flags().StringVarP(&cfg.Flags.Path, "filename", "f", "", "filename path")
	_ = c.MarkFlagRequired("filename")

	return c
}
