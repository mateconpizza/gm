package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/presenter"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util/files"
	"github.com/haaag/gm/pkg/util/frame"
)

var (
	dbDrop   bool
	dbInfo   bool
	dbList   bool
	dbRemove bool
)

var ErrEmptyString = errors.New("empty string")

// dbExistsAndInit checks if the default database exists and is initialized.
func dbExistsAndInit(path, name string) bool {
	f := filepath.Join(path, files.EnsureExtension(name, ".db"))
	return dbExists(f) && isInitialized(f)
}

// isInitialized checks if the database is initialized.
func isInitialized(f string) bool {
	return files.Size(f) > 0
}

// dbExists checks if a database exists.
func dbExists(f string) bool {
	return files.Exists(f)
}

// getDBs returns the list of databases from the given path.
func getDBs(path string) ([]string, error) {
	f, err := files.FindByExtension(path, "db")
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return f, nil
}

// repoInfo returns the repository info.
func repoInfo(r *repo.SQLiteRepository) string {
	main := r.GetMaxID(r.Cfg.GetTableMain())
	deleted := r.GetMaxID(r.Cfg.GetTableDeleted())
	header := color.Yellow(r.Cfg.Name).Bold().Italic().String()

	f := frame.New(frame.WithColorBorder(color.Yellow)).Header(header)
	f.Row(format.BulletLine("records:", strconv.Itoa(main))).
		Row(format.BulletLine("deleted:", strconv.Itoa(deleted))).
		Row(format.BulletLine("backup status:", getBackupStatus(r.Cfg.MaxBackups))).
		Footer(format.BulletLine("path:", r.Cfg.Path))

	return f.String()
}

// handleDBDrop clears the database.
func handleDBDrop(r *Repo) error {
	if !r.IsInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.Name)
	}

	if r.IsEmpty(r.Cfg.GetTableMain(), r.Cfg.GetTableDeleted()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBEmpty, r.Cfg.Name)
	}

	fmt.Println(repoInfo(r))

	q := fmt.Sprintf("remove %s bookmarks?", color.Red("all").Bold())
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
func removeDB(r *Repo) error {
	// FIX: redo this function.
	var (
		n        int
		bks      []string
		info     = presenter.RepoSummary(r)
		question = fmt.Sprintf("remove %s?", color.Red(r.Cfg.Name).Bold())
	)

	bks, _ = getBackupList(r.Cfg.BackupPath, r.Cfg.Name)
	n = len(bks)

	if n > 0 {
		info += presenter.BackupsSummary(r)
	}
	fmt.Println(info)

	if !terminal.Confirm(question, "n") {
		return ErrActionAborted
	}

	if err := files.Remove(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	if n > 0 {
		for _, s := range bks {
			f := filepath.Base(s)
			q := fmt.Sprintf("remove %s?", color.Red(f).Bold())
			if terminal.Confirm(q, "n") {
				if err := files.Remove(s); err != nil {
					return fmt.Errorf("%w", err)
				}
			}
		}
	}

	fmt.Println(color.Green("database and/or backups removed successfully"))

	return nil
}

// checkDBState verifies database existence and initialization.
func checkDBState(f string) error {
	if !dbExists(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, f)
	}
	if !isInitialized(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, f)
	}

	return nil
}

// handleListDB lists the available databases.
func handleListDB(r *Repo) error {
	databases, err := getDBs(r.Cfg.Path)
	if err != nil {
		return err
	}

	n := len(databases)
	if n == 0 {
		return fmt.Errorf("%w", repo.ErrDBsNotFound)
	}

	f := frame.New(frame.WithColorBorder(color.Gray))

	// add header
	if n > 1 {
		nColor := color.BrightCyan(n).Bold().String()
		f.Header(nColor + " database/s found").Newline()
	}

	lastIdx := len(databases) - 1
	for i, db := range databases {
		name := filepath.Base(db)
		Cfg.SetName(name)
		rep, _ := repo.New(Cfg)
		f.Text(presenter.RepoSummary(rep))
		if i != lastIdx {
			f.Newline()
		}
	}

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
func handleNewDB(r *Repo) error {
	if dbExists(r.Cfg.Fullpath()) && r.IsInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyExists, r.Cfg.Name)
	}

	if !DBInit {
		init := color.Yellow("--init").Bold().Italic()
		return fmt.Errorf("%w: use %s", repo.ErrDBNotInitialized, init)
	}

	return handleDBInit()
}

// handleRemoveDB removes a database.
func handleRemoveDB(r *Repo) error {
	if !dbExists(r.Cfg.Fullpath()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, r.Cfg.Name)
	}

	return removeDB(r)
}

// handleDBInfo prints information about a database.
func handleDBInfo(r *Repo) error {
	if JSON {
		fmt.Println(string(format.ToJSON(r)))
		return nil
	}

	s := presenter.RepoSummary(r)
	s += presenter.BackupDetail(r)
	fmt.Print(s)

	return nil
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "database management",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}

		flags := map[bool]func(r *Repo) error{
			dbDrop:   handleDBDrop,
			dbInfo:   handleDBInfo,
			dbList:   handleListDB,
			dbRemove: handleRemoveDB,
			DBInit:   handleNewDB,
		}
		if handler, ok := flags[true]; ok {
			return handler(r)
		}

		return handleDBInfo(r)
	},
}

func init() {
	dbCmd.Flags().BoolVarP(&dbDrop, "drop", "d", false, "drop a database")
	dbCmd.Flags().BoolVarP(&dbInfo, "info", "I", false, "show database info (default)")
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVarP(&dbRemove, "remove", "r", false, "remove a database")
	rootCmd.AddCommand(dbCmd)
}
