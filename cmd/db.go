// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

var (
	dbCreate string
	dbInit   bool
	dbList   bool
	dbRemove string
)

// addSuffix adds .db to the database name
func addSuffix(name string) string {
	if !strings.HasSuffix(name, ".db") {
		name = fmt.Sprintf("%s.db", name)
	}
	return name
}

// isInitialized checks if the database is initialized
func isInitialized(name string) bool {
	f := config.App.Path.Home + "/" + name
	size := config.Filesize(f)
	return size > 0
}

// dbExists checks if a database exists
func dbExists(name string) bool {
	file := config.App.Path.Home + "/" + name
	return config.FileExists(file)
}

// getDBName determines the database name from the arguments
func getDBName(args []string) string {
	if len(args) == 0 {
		return config.DefaultDB
	}
	return addSuffix(strings.ToLower(args[0]))
}

// removeSecure removes a database
func removeSecure(name string) error {
	if !dbExists(name) {
		return fmt.Errorf("%w: '%s'", bookmark.ErrDBNotFound, name)
	}
	f := config.App.Path.Home + "/" + name
	if err := os.Remove(f); err != nil {
		return fmt.Errorf("removing database: %w", err)
	}
	return nil
}

// removeDB removes a database
func removeDB(name string) error {
	q := fmt.Sprintf("Remove %s database?", format.Text(name).Red().Bold())
	option := promptWithOptions(q, []string{"Yes", "No"})
	switch option {
	case "n", "no", "No":
		return nil
	case "y", "yes", "Yes":
		if err := removeSecure(name); err != nil {
			return fmt.Errorf("%w", err)
		}
		fmt.Println(format.Text("Database removed successfully.").Green())
	}
	return nil
}

// checkDBState verifies database existence and initialization
func checkDBState(name string) error {
	if !dbExists(name) {
		return fmt.Errorf("%w: '%s'", bookmark.ErrDBNotFound, name)
	}

	if !isInitialized(name) {
		return fmt.Errorf("%w", bookmark.ErrDBNotInitialized)
	}

	return nil
}

// handleListDB lists the available databases
func handleListDB() error {
	databases := make([]string, 0)

	files, err := filepath.Glob(config.App.Path.Home + "/*.db")
	if err != nil {
		return fmt.Errorf("listing databases: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("%w", bookmark.ErrDBsNotFound)
	}

	for _, f := range files {
		file := filepath.Base(f)
		databases = append(databases, format.BulletLine(file, ""))
	}

	config.Version()
	t := format.Text("\ndatabase/s found").Yellow().String()
	s := format.HeaderWithSection(t, databases)
	fmt.Print(s)
	return nil
}

// handleDBInit initializes the database
func handleDBInit(args []string) error {
	name := getDBName(args)
	dbName = name
	if err := initCmd.RunE(nil, []string{}); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// handleNewDB creates and initializes a new database
func handleNewDB(s string) error {
	s = addSuffix(s)
	if dbExists(s) {
		return fmt.Errorf("%w: %s", bookmark.ErrDBAlreadyExists, s)
	}

	if !dbInit {
		init := format.Text("--init").Yellow().Bold()
		return fmt.Errorf("%w: use %s", bookmark.ErrDBNotInitialized, init)
	}

	if err := handleDBInit([]string{s}); err != nil {
		return fmt.Errorf("%w", err)
	}
	return nil
}

// handleRemoveDB removes a database
func handleRemoveDB(s string) error {
	s = addSuffix(s)
	if !dbExists(s) {
		return fmt.Errorf("%w: '%s'", bookmark.ErrDBNotFound, s)
	}
	if err := removeDB(s); err != nil {
		return fmt.Errorf("removing database: %w", err)
	}
	return nil
}

// handleDBInfo prints information about a database
func handleDBInfo(args []string) error {
	name := getDBName(args)

	if err := checkDBState(name); err != nil {
		return err
	}

	r, err := bookmark.NewRepository(name)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	handleAppInfo(r)
	return nil
}

// dbUsage returns the usage of the db command
func dbUsage() string {
	s := `Usage:
  %s db <name> [flags]
  %s db --list

Flags:
  -l, --list            list available databases
  -c, --create <name>   create a new database
  -i, --init   <name>   initialize a new database
  -r, --remove <name>   remove a database

Global Flags:
  --color string        print with pretty colors [always|never]
  --json                print data in JSON format`
	s += "\n"
	c := config.App.Cmd
	return fmt.Sprintf(s, c, c)
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: fmt.Sprintf("%s\n\n%s", config.AppBanner, "bookmarks database management"),
	PreRun: func(cmd *cobra.Command, args []string) {
		config.LoadAppPaths()
	},
	RunE: func(_ *cobra.Command, args []string) error {
		if dbList {
			return handleListDB()
		}
		if dbCreate != "" {
			return handleNewDB(dbCreate)
		}
		if dbRemove != "" {
			return handleRemoveDB(dbRemove)
		}
		if dbInit {
			return handleDBInit(args)
		}
		return handleDBInfo(args)
	},
}

func init() {
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVarP(&dbInit, "init", "i", false, "initialize a new database")
	dbCmd.Flags().StringVarP(&dbCreate, "create", "c", "", "create a new database")
	dbCmd.Flags().StringVarP(&dbRemove, "remove", "r", "", "remove a database")
	dbCmd.SetUsageTemplate(dbUsage())
	rootCmd.AddCommand(dbCmd)
}
