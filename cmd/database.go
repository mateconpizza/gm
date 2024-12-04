package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	dbCreate bool
	dbDrop   bool
	dbInfo   bool
	dbList   bool
	dbRemove bool
)

// handleDBDrop clears the database.
func handleDBDrop(r *repo.SQLiteRepository) error {
	if !r.IsDatabaseInitialized(r.Cfg.TableMain) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.Name)
	}

	if r.IsEmpty(r.Cfg.TableMain, r.Cfg.TableDeleted) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBEmpty, r.Cfg.Name)
	}

	fmt.Print(repo.Info(r))

	q := fmt.Sprintf("\nremove %s bookmarks?", color.Red("all").Bold())
	if !terminal.Confirm(q, "n") {
		return ErrActionAborted
	}

	if err := r.DropSecure(); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("Successfully").Italic().Bold()
	fmt.Printf("%s database cleared\n", success)

	return nil
}

// removeDB removes a database.
func removeDB(r *repo.SQLiteRepository) error {
	fmt.Print(repo.Info(r))

	q := fmt.Sprintf("\nremove %s?", color.Red(r.Cfg.Name).Bold())
	if !terminal.Confirm(q, "n") {
		return ErrActionAborted
	}

	backups, _ := repo.Backups(r)
	n := backups.Len()

	if n > 0 {
		q = fmt.Sprintf("remove %d %s?", n, color.Red("backup/s").Bold())
		if !terminal.Confirm(q, "n") {
			return ErrActionAborted
		}

		if err := backups.ForEachErr(repo.Remove); err != nil {
			return fmt.Errorf("removing backup: %w", err)
		}
	}

	// remove repo
	if err := repo.Remove(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	success := color.BrightGreen("Successfully").Italic().Bold()
	fmt.Printf("%s database removed\n", success)

	return nil
}

// checkDBState verifies database existence and initialization.
func checkDBState(f string) error {
	if !files.Exists(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, f)
	}
	if files.IsEmpty(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, f)
	}

	return nil
}

// handleListDB lists the available databases.
func handleListDB(r *repo.SQLiteRepository) error {
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

// handleRemoveDB removes a database.
func handleRemoveDB(r *repo.SQLiteRepository) error {
	if err := r.Cfg.Exists(); err != nil {
		return fmt.Errorf("%w: '%s'", err, r.Cfg.Name)
	}

	return removeDB(r)
}

// handleDBInfo prints information about a database.
func handleDBInfo(r *repo.SQLiteRepository) error {
	if JSON {
		backups, _ := repo.Backups(r)
		r.Cfg.Backup.Files = *backups.Items()
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
			dbDrop:   handleDBDrop,
			dbInfo:   handleDBInfo,
			dbList:   handleListDB,
			dbRemove: handleRemoveDB,
		}
		if handler, ok := flags[true]; ok {
			return handler(r)
		}

		return handleDBInfo(r)
	},
}

func init() {
	dbCmd.Flags().BoolVarP(&dbCreate, "create", "c", false, "create a new database")
	dbCmd.Flags().BoolVarP(&dbDrop, "drop", "d", false, "drop a database")
	dbCmd.Flags().BoolVarP(&dbInfo, "info", "I", false, "show database info (default)")
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVarP(&dbRemove, "remove", "r", false, "remove a database")
	dbCmd.AddCommand(initCmd)
	rootCmd.AddCommand(dbCmd)
}
