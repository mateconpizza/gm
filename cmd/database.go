package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/terminal"
)

// Subcommand Flags.
var dbDrop, dbInfo, dbList bool

// dbDropHandler clears the database.
func dbDropHandler(r *repo.SQLiteRepository) error {
	if !r.IsDatabaseInitialized(r.Cfg.Tables.Main) {
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

	if !terminal.Confirm(f.Footer("continue?").String(), "n") {
		return handler.ErrActionAborted
	}

	if err := r.DropSecure(); err != nil {
		return fmt.Errorf("%w", err)
	}

	terminal.ClearLine(1)
	success := color.BrightGreen("Successfully").Italic().String()
	f.Clean().Success(success + " database dropped").Ln().Render()

	return nil
}

// dbRemoveHandler removes a database.
func dbRemoveHandler(r *repo.SQLiteRepository) error {
	if !r.Cfg.Exists() {
		return repo.ErrDBNotFound
	}
	f := frame.New(frame.WithColorBorder(color.BrightGray), frame.WithNoNewLine())

	i := repo.Info(r)
	i += f.Row().Ln().String()
	fmt.Print(i)

	f.Clean().Mid(fmt.Sprintf("remove %s?", color.Red(r.Cfg.Name)))
	if !terminal.Confirm(f.String(), "n") {
		return handler.ErrActionAborted
	}

	var n int
	backups, err := repo.Backups(r)
	if err != nil {
		log.Printf("removeDB: %s", err)
		n = 0
	} else {
		n = backups.Len()
	}

	linesToClear := 1
	if n > 0 {
		linesToClear++
		f.Clean().Mid(fmt.Sprintf("remove %d %s?", n, color.Red("backup/s")))
		if !terminal.Confirm(f.String(), "n") {
			return handler.ErrActionAborted
		}

		if err := backups.ForEachErr(repo.Remove); err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}
	}

	// remove repo
	if err := repo.Remove(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	terminal.ClearLine(linesToClear)
	success := color.BrightGreen("Successfully").Italic().String()
	f.Clean().Success(success + " database removed").Ln().Render()

	return nil
}

// dbListHandler lists the available databases.
func dbListHandler(r *repo.SQLiteRepository) error {
	dbs, err := repo.Databases(r.Cfg)
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

	dbs.ForEachIdx(func(i int, r *repo.SQLiteRepository) {
		f.Text(repo.Summary(r))
	})

	f.Render()

	return nil
}

// dbInfoPrint prints information about a database.
func dbInfoPrint(r *repo.SQLiteRepository) error {
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
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return verifyDatabase(Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}

		flags := map[bool]func(r *repo.SQLiteRepository) error{
			dbDrop: dbDropHandler,
			dbInfo: dbInfoPrint,
			dbList: dbListHandler,
			Remove: dbRemoveHandler,
		}
		if handler, ok := flags[true]; ok {
			return handler(r)
		}

		return dbInfoPrint(r)
	},
}

func init() {
	dbCmd.Flags().BoolVarP(&dbDrop, "drop", "d", false, "drop a database")
	dbCmd.Flags().BoolVarP(&dbInfo, "info", "I", false, "show database info (default)")
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVarP(&Remove, "remove", "r", false, "remove a database")
	dbCmd.Flags().BoolVar(&JSON, "json", false, "output in JSON format")
	dbCmd.AddCommand(initCmd)
	rootCmd.AddCommand(dbCmd)
}
