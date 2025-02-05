package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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

// removeDatabases removes a list of databases.
func removeDatabases(t *terminal.Term, dbs *slice.Slice[Repo]) error {
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
	s := color.BrightRed("removing").String()

	dbs.ForEachIdx(func(i int, r Repo) {
		f.Info(repo.SummaryRecordsLine(&r)).Ln()
	})

	msg := s + " " + strconv.Itoa(dbs.Len()) + " items/s"
	if !t.Confirm(f.Row("\n").Warning(msg+", continue?").String(), "n") {
		return handler.ErrActionAborted
	}

	sp := spinner.New(spinner.WithMesg(color.Yellow("removing database...").String()))
	sp.Start()
	if err := dbs.ForEachErr(removeSecure); err != nil {
		return fmt.Errorf("%w", err)
	}

	sp.Stop()
	t.ClearLine(format.CountLines(f.String()))
	s = color.BrightGreen("Successfully").Italic().String()
	f.Clean().Success(s + " " + strconv.Itoa(dbs.Len()) + " items/s removed").Ln().Render()

	return nil
}

// dbDropCmd drops a database.
var dbDropCmd = &cobra.Command{
	Use:   "drop",
	Short: "drop a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()

		if !r.IsInitialized() {
			return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.Name)
		}

		if r.IsEmpty(r.Cfg.Tables.Main, r.Cfg.Tables.Deleted) {
			return fmt.Errorf("%w: '%s'", repo.ErrDBEmpty, r.Cfg.Name)
		}

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		warn := color.BrightRed("dropping").String()
		f.Header(warn + " all bookmarks database").Ln().Row().Ln().Render()
		fmt.Print(repo.Info(r))
		f.Clean().Row().Ln().Render().Clean()
		if !Force {
			if !t.Confirm(f.Footer("continue?").String(), "n") {
				return handler.ErrActionAborted
			}
		}

		if err := r.DropSecure(context.Background()); err != nil {
			return fmt.Errorf("%w", err)
		}

		if !Verbose {
			t.ClearLine(1)
		}
		success := color.BrightGreen("Successfully").Italic().String()
		f.Clean().Success(success + " database dropped").Ln().Render()

		return nil
	},
}

// dbDropCmd drops a database.
var dbListCmd = &cobra.Command{
	Use:     "list",
	Short:   "list databases",
	Aliases: []string{"ls", "l"},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		dbs, err := repo.Databases(r.Cfg.Path)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		n := dbs.Len()
		if n == 0 {
			return fmt.Errorf("%w", repo.ErrDBsNotFound)
		}

		f := frame.New(frame.WithColorBorder(color.Gray))
		// add header
		if n > 1 {
			nColor := color.BrightCyan(n).Bold().String()
			f.Header(nColor + " database/s found").Ln()
		}

		dbs.ForEachMut(func(r *Repo) {
			f.Text(repo.Summary(r))
		})

		f.Render()

		return nil
	},
}

// dbInfoCmd shows information about a database.
var dbInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "show information about a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()
		if JSON {
			backups, err := repo.Backups(r)
			if err != nil {
				Cfg.Backup.Files = nil
			} else {
				backups.ForEach(func(bk Repo) {
					Cfg.Backup.Files = append(Cfg.Backup.Files, bk.Cfg.Fullpath())
				})
			}
			fmt.Println(string(format.ToJSON(r)))

			return nil
		}

		fmt.Print(repo.Info(r))

		return nil
	},
}

// dbRemoveCmd remove a database.
var dbRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm", "del", "delete"},
	Short:   "remove a database",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		if !r.Cfg.Exists() {
			return repo.ErrDBNotFound
		}
		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))
		f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())
		// show repo info
		i := repo.Info(r)
		i += f.Row().Ln().String()
		fmt.Print(i)

		rm := color.BrightRed("remove").Bold().String()
		if !t.Confirm(f.Clean().Mid(rm+" "+r.Cfg.Name+"?").String(), "n") {
			return handler.ErrActionAborted
		}
		filesToRemove := slice.New[Repo]()

		backups, err := repo.Backups(r)
		if err != nil {
			if !errors.Is(err, repo.ErrBackupNotFound) {
				return fmt.Errorf("%w", err)
			}
		}
		// add backups to remove
		if backups.Len() > 0 {
			items := backups.Items()
			f.Clean().Mid(rm + " backups?")
			opt := t.Choose(f.String(), []string{"all", "no", "select"}, "n")
			switch strings.ToLower(opt) {
			case "n", "no":
				sk := color.BrightYellow("skipping").String()
				t.ReplaceLine(1, f.Clean().Warning(sk+" backup/s\n").Row().String())
			case "a", "all":
				filesToRemove.Set(items)
			case "s", "select":
				m := menu.New[Repo](
					menu.WithDefaultSettings(),
					menu.WithMultiSelection(),
					menu.WithHeader(fmt.Sprintf("select backup/s from '%s'", r.Cfg.Name), false),
					menu.WithPreviewCustomCmd(config.App.Cmd+" db -n ./backup/{1} info"),
				)
				selected, err := handler.Selection(m, items, nil)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
				t.ClearLine(1)
				filesToRemove.Append(selected...)
			}
		}
		filesToRemove.Push(r)

		return removeDatabases(t, filesToRemove)
	},
}

// dbCmd database management.
var dbCmd = &cobra.Command{
	Use:     "database",
	Aliases: []string{"db"},
	Short:   "database management",
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

func init() {
	f := dbCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	f.StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	dbInfoCmd.Flags().BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	_ = dbCmd.Flags().MarkHidden("color")
	// add subcommands
	dbCmd.AddCommand(dbDropCmd)
	dbCmd.AddCommand(dbInfoCmd)
	dbCmd.AddCommand(dbInitCmd)
	dbCmd.AddCommand(dbListCmd)
	dbCmd.AddCommand(dbRemoveCmd)
	rootCmd.AddCommand(dbCmd)
}
