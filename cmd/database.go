package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util/files"
	"github.com/haaag/gm/internal/util/frame"
)

var (
	dbCreate bool
	dbDrop   bool
	dbInfo   bool
	dbList   bool
	dbRemove bool
)

// dbExistsAndInit checks if the default database exists and is initialized.
func dbExistsAndInit(path, name string) bool {
	f := filepath.Join(path, files.EnsureExtension(name, ".db"))
	return repo.Exists(f) && repo.IsNonEmptyFile(f)
}

// handleDBDrop clears the database.
func handleDBDrop(r *repo.SQLiteRepository) error {
	if !r.IsDatabaseInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.Name)
	}

	if r.IsEmpty(r.Cfg.GetTableMain(), r.Cfg.GetTableDeleted()) {
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

	success := color.BrightGreen("successfully").Italic().Bold()
	fmt.Println("database cleared", success.String())

	return nil
}

// removeDB removes a database.
func removeDB(r *repo.SQLiteRepository) error {
	fmt.Print(repo.Info(r))

	q := fmt.Sprintf("\nremove %s?", color.Red(r.Cfg.Name).Bold())
	if !terminal.Confirm(q, "n") {
		return ErrActionAborted
	}

	backups, _ := repo.GetBackups(r)
	n := backups.Len()

	q = fmt.Sprintf("remove %d %s?", n, color.Red("backup/s").Bold())
	if !terminal.Confirm(q, "n") {
		return ErrActionAborted
	}

	/* backups.ForEach(func(b string) {
		fmt.Println(b)
	}) */

	success := color.BrightGreen("successfully").Italic().Bold()
	fmt.Println("\ndatabase removed", success)

	return nil
}

// checkDBState verifies database existence and initialization.
func checkDBState(f string) error {
	if !repo.Exists(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, f)
	}
	if !repo.IsNonEmptyFile(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, f)
	}

	return nil
}

// handleListDB lists the available databases.
func handleListDB(r *repo.SQLiteRepository) error {
	databases, err := repo.GetDatabases(r.Cfg)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	n := databases.Len()
	if n == 0 {
		return fmt.Errorf("%w", repo.ErrDBsNotFound)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))
	// add header
	if n > 1 {
		nColor := color.BrightCyan(n).Bold().String()
		f.Header(nColor + " database/s found").Newline()
	}

	databases.ForEachIdx(func(i int, r *repo.SQLiteRepository) {
		f.Text(repo.Summary(r))
	})

	f.Render()

	return nil
}

// handleDBInit initializes the database.
func handleDBInit() error {
	if !DBInit {
		return nil
	}

	if err := initCmd.RunE(nil, []string{}); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

// handleNewDB creates and initializes a new database.
func handleNewDB(r *repo.SQLiteRepository) error {
	if repo.Exists(r.Cfg.Fullpath()) && r.IsDatabaseInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyExists, r.Cfg.Name)
	}

	if !DBInit {
		init := color.Yellow("--init").Bold().Italic()
		return fmt.Errorf("%w: use %s", repo.ErrDBNotInitialized, init)
	}

	return handleDBInit()
}

// handleRemoveDB removes a database.
func handleRemoveDB(r *repo.SQLiteRepository) error {
	if !repo.Exists(r.Cfg.Fullpath()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, r.Cfg.Name)
	}

	return removeDB(r)
}

// handleDBInfo prints information about a database.
func handleDBInfo(r *repo.SQLiteRepository) error {
	if JSON {
		fmt.Println(string(format.ToJSON(r)))
		return nil
	}

	fmt.Print(repo.Info(r))

	return nil
}

func handleCreateDB(_ *repo.SQLiteRepository) error {
	DBInit = true

	return handleDBInit()
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "database management",
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
			dbCreate: handleCreateDB,
			DBInit:   handleNewDB,
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
	rootCmd.AddCommand(dbCmd)
}
