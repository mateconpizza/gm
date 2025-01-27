package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Subcommand Flags.
var dbDrop, dbInfo, dbList bool

// dbDropHandler clears the database.
func dbDropHandler(t *terminal.Term, r *repo.SQLiteRepository) error {
	if !r.IsInitialized() {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.Name)
	}

	if r.IsEmpty(r.Cfg.Tables.Main, r.Cfg.Tables.Deleted) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBEmpty, r.Cfg.Name)
	}

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
}

// dbRemoveHandler removes a database.
func dbRemoveHandler(t *terminal.Term, r *repo.SQLiteRepository) error {
	if !r.Cfg.Exists() {
		return repo.ErrDBNotFound
	}
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())

	i := repo.Info(r)
	i += f.Row().Ln().String()
	fmt.Print(i)

	f.Clean().Mid(fmt.Sprintf("remove %s?", color.Red(r.Cfg.Name)))
	if !t.Confirm(f.String(), "n") {
		return handler.ErrActionAborted
	}

	var n int
	backups, err := repo.Backups(r)
	if err != nil {
		log.Printf("dbRemoveHandler: %s", err)
		n = 0
	} else {
		n = backups.Len()
	}

	linesToClear := 1
	if n > 0 {
		linesToClear++
		f.Clean().Mid(fmt.Sprintf("remove %d %s?", n, color.Red("backup/s")))
		if !t.Confirm(f.String(), "n") {
			return handler.ErrActionAborted
		}

		if err := backups.ForEachErr(files.Remove); err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}
	}

	// remove repo
	if err := files.Remove(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	t.ClearLine(linesToClear)
	success := color.BrightGreen("Successfully").Italic().String()
	f.Clean().Success(success + " database removed").Ln().Render()

	return nil
}

// dbListHandler lists the available databases.
func dbListHandler(_ *terminal.Term, r *repo.SQLiteRepository) error {
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

	dbs.ForEachIdx(func(i int, r repo.SQLiteRepository) {
		f.Text(repo.Summary(&r))
	})

	f.Render()

	return nil
}

// dbInfoPrint prints information about a database.
//
//nolint:unparam //ignore
func dbInfoPrint(_ *terminal.Term, r *repo.SQLiteRepository) error {
	if JSON {
		backups, err := repo.Backups(r)
		if err != nil {
			Cfg.Backup.Files = nil
		} else {
			Cfg.Backup.Files = *backups.Items()
		}
		fmt.Println(string(format.ToJSON(r)))

		return nil
	}

	fmt.Print(repo.Info(r))

	return nil
}

var dbCmd = &cobra.Command{
	Use:     "db",
	Aliases: []string{"database"},
	Short:   "database management",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		defer r.Close()

		t := terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))

		flags := map[bool]func(t *terminal.Term, r *repo.SQLiteRepository) error{
			dbDrop: dbDropHandler,
			dbInfo: dbInfoPrint,
			dbList: dbListHandler,
			Remove: dbRemoveHandler,
		}
		if handler, ok := flags[true]; ok {
			return handler(t, r)
		}

		return dbInfoPrint(t, r)
	},
}

func init() {
	f := dbCmd.Flags()
	f.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	f.StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	f.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	// actions
	f.BoolVarP(&dbDrop, "drop", "d", false, "drop a database")
	f.BoolVarP(&dbInfo, "info", "i", false, "output database info")
	f.BoolVarP(&dbList, "list", "l", false, "list available databases")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a database")

	_ = dbCmd.Flags().MarkHidden("color")
	dbCmd.AddCommand(initCmd)
	rootCmd.AddCommand(dbCmd)
}
