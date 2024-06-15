package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
	"github.com/haaag/gm/pkg/util"
)

var (
	dbDrop   bool
	dbInfo   bool
	dbList   bool
	dbRemove bool
)

var ErrEmptyString = errors.New("empty string")

// dbExistsAndInit checks if the default database exists and is initialized
func dbExistsAndInit(path, name string) bool {
	f := filepath.Join(path, ensureDbSuffix(name))
	return dbExists(f) && isInitialized(f)
}

// ensureDbSuffix adds .db to the database name
func ensureDbSuffix(name string) string {
	suffix := ".db"
	if !strings.HasSuffix(name, suffix) {
		name = fmt.Sprintf("%s%s", name, suffix)
	}
	return name
}

// isInitialized checks if the database is initialized
func isInitialized(f string) bool {
	return util.Filesize(f) > 0
}

// dbExists checks if a database exists
func dbExists(f string) bool {
	return util.FileExists(f)
}

// getDBNameFromArgs determines the database name from the arguments
func getDBNameFromArgs(args []string) string {
  // FIX: delete me...
	if len(args) == 0 {
		return app.DefaultDBName
	}
	return ensureDbSuffix(strings.ToLower(args[0]))
}

// getDBs returns the list of databases
func getDBs(path string) ([]string, error) {
	var files []string
	if err := util.FilesWithSuffix(path, "db", &files); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return files, nil
}

// getDBsBasename returns the basename
func getDBsBasename(f []string) []string {
	b := make([]string, 0, len(f))
	for _, v := range f {
		b = append(b, format.BulletLine(filepath.Base(v), ""))
	}
	return b
}

// repoInfo prints information about a database
func repoInfo(r *repo.SQLiteRepository) string {
	main := r.GetMaxID(r.Cfg.GetTableMain())
	deleted := r.GetMaxID(r.Cfg.GetTableDeleted())
	t := format.Color(r.Cfg.GetName()).Yellow().Bold().String()
	return format.HeaderWithSection(t, []string{
		format.BulletLine("records:", strconv.Itoa(main)),
		format.BulletLine("deleted:", strconv.Itoa(deleted)),
		format.BulletLine("backup status:", getBkStateColored(r.Cfg.Backup.GetMax())),
		format.BulletLine("path:", r.Cfg.GetHome()),
	})
}

// handleDBDrop clears the database
func handleDBDrop(r *Repository) error {
	if !r.IsInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, r.Cfg.GetName())
	}

	if r.IsEmpty(r.Cfg.GetTableMain(), r.Cfg.GetTableDeleted()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBEmpty, r.Cfg.GetName())
	}

	fmt.Println(repoInfo(r))

	q := fmt.Sprintf("remove %s bookmarks?", format.Color("all").Red().Bold())
	if !terminal.Confirm(q, "n") {
		return ErrActionAborted
	}

	if err := r.DropSecure(); err != nil {
		return fmt.Errorf("%w", err)
	}

	fmt.Println(format.Color("database cleared successfully").Green())
	return nil
}

// removeDB removes a database
func removeDB(r *Repository) error {
	var (
		n        = len(r.Cfg.Backup.List())
		info     = repoInfo(r)
		color    = format.Color
		question = fmt.Sprintf("remove %s?", color(r.Cfg.GetName()).Red().Bold())
	)

	if n > 0 {
		info += "\n" + backupInfo(r)
	}
	fmt.Println(info)

	if !terminal.Confirm(question, "n") {
		return ErrActionAborted
	}

	if err := util.RmFile(r.Cfg.Fullpath()); err != nil {
		return fmt.Errorf("%w", err)
	}

	if n > 0 {
		for _, s := range r.Cfg.Backup.List() {
			f := filepath.Base(s)
			q := fmt.Sprintf("remove %s?", color(f).Red().Bold())
			if terminal.Confirm(q, "n") {
				if err := util.RmFile(s); err != nil {
					return fmt.Errorf("%w", err)
				}
			}
		}
	}
	fmt.Println(color("database and/or backups removed successfully").Green())
	return nil
}

// checkDBState verifies database existence and initialization
func checkDBState(f string) error {
	if !dbExists(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, f)
	}

	if !isInitialized(f) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotInitialized, f)
	}

	return nil
}

// handleListDB lists the available databases
func handleListDB(r *Repository) error {
	var sb strings.Builder
	files, err := getDBs(r.Cfg.GetHome())
	if err != nil {
		return err
	}

	var n = len(files)
	if n == 0 {
		return fmt.Errorf("%w", repo.ErrDBsNotFound)
	}

	if n > 1 {
		m := fmt.Sprintf("listing %d database/s found", n)
		sb.WriteString(format.Header(m))
	}

	// TODO: format in a better way
	for i, db := range files {
		name := filepath.Base(db)
		Cfg.SetName(name)
		rep, _ := repo.New(Cfg)
		sb.WriteString(repoInfo(rep))
		if i != n-1 {
			sb.WriteString("\n")
		}
	}

	fmt.Print(sb.String())
	return nil
}

// handleDBInit initializes the database
func handleDBInit() error {
	if err := initCmd.RunE(nil, []string{}); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// handleNewDB creates and initializes a new database
func handleNewDB(r *Repository) error {
	if dbExists(r.Cfg.Fullpath()) && r.IsInitialized(r.Cfg.GetTableMain()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBAlreadyExists, r.Cfg.GetName())
	}

	if !DBInit {
		init := format.Color("--init").Yellow().Bold()
		return fmt.Errorf("%w: use %s", repo.ErrDBNotInitialized, init)
	}

	return handleDBInit()
}

// handleRemoveDB removes a database
func handleRemoveDB(r *Repository) error {
	if !dbExists(r.Cfg.Fullpath()) {
		return fmt.Errorf("%w: '%s'", repo.ErrDBNotFound, r.Cfg.GetName())
	}
	return removeDB(r)
}

// handleDBInfo prints information about a database
func handleDBInfo(r *Repository) error {
	if Json {
		fmt.Println(string(format.ToJSON(r)))
		return nil
	}

	s := repoInfo(r) + "\n"
	s += backupInfo(r)
	fmt.Println(s)
	return nil
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "bookmarks database management",
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("database: %w", err)
		}
		if err := loadBackups(r); err != nil {
			return err
		}

		flags := map[bool]func(r *Repository) error{
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
	dbCmd.Flags().BoolVarP(&dbInfo, "info", "I", false, "show database info")
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVarP(&dbRemove, "remove", "r", false, "remove a database")
	rootCmd.AddCommand(dbCmd)
}
