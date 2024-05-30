// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	// dbCreate bool
	dbInit bool
	dbList bool
	// dbRemove bool
	dbCreate string
	dbRemove string
)

var ErrDBSpecify = errors.New("specify a database name")

// dbExists checks if a database exists
func dbExists(name string) bool {
	config.LoadAppPaths()
	files, _ := filepath.Glob(config.App.Path.Home + "/*.db")
	for _, f := range files {
		if strings.EqualFold(filepath.Base(f), name) {
			return true
		}
	}
	return false
}

// removeDB removes a database
func removeDB(name string) error {
	if !dbExists(name) {
		return fmt.Errorf("%w: %s", bookmark.ErrDBNotFound, name)
	}
	config.LoadAppPaths()

	f := config.App.Path.Home + "/" + name
	q := fmt.Sprintf("Remove database: '%s'?", name)

	option := promptWithOptions(q, []string{"Yes", "No"})
	switch option {
	case "n", "no", "No":
		return fmt.Errorf("%w", bookmark.ErrActionAborted)
	case "y", "yes", "Yes":
		if err := os.Remove(f); err != nil {
			return fmt.Errorf("removing database: %w", err)
		}
		fmt.Println(format.Text("\ndatabase removed successfully.").Green())
	}

	return nil
}

// handleListDB lists the available databases
func handleListDB() error {
	databases := make([]string, 0)
	config.LoadAppPaths()

	files, err := filepath.Glob(config.App.Path.Home + "/*.db")
	if err != nil {
		return fmt.Errorf("listing databases: %w", err)
	}

	for _, f := range files {
		file := filepath.Base(f)
		databases = append(databases, format.BulletLine(file, ""))
	}

	appName := format.Text(config.App.Name).Green()
	s := fmt.Sprintf("%s v%s\n\n", appName, config.App.Version)
	t := format.Text("database/s found").Yellow().String()
	s += format.HeaderWithSection(t, databases)
	fmt.Println(s)
	return nil
}

// handleNewDB creates and initializes a new database
func handleNewDB() error {
	if !dbInit {
		init := format.Text("--init").Yellow().Bold()
		return fmt.Errorf("%w: use %s", bookmark.ErrDBInit, init)
	}

	if !strings.HasSuffix(dbCreate, ".db") {
		dbCreate = fmt.Sprintf("%s.db", dbCreate)
	}
	dbName = dbCreate
	if err := initCmd.RunE(nil, []string{}); err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	return nil
}

// handleRemoveDB removes a database
func handleRemoveDB() error {
	if !strings.HasSuffix(dbRemove, ".db") {
		dbRemove = fmt.Sprintf("%s.db", dbRemove)
	}
	if !dbExists(dbRemove) {
		return fmt.Errorf("%w: %s", bookmark.ErrDBNotFound, dbRemove)
	}
	if err := removeDB(dbRemove); err != nil {
		return fmt.Errorf("removing database: %w", err)
	}
	return nil
}

// handleDBInfo prints information about a database
func handleDBInfo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%w", ErrDBSpecify)
	}
	name := strings.ToLower(args[0])
	r, err := bookmark.NewRepository(name)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	handleAppInfo(r)
	return nil
}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "database management",
	RunE: func(_ *cobra.Command, args []string) error {
		if dbList {
			return handleListDB()
		} else if dbCreate != "" {
			return handleNewDB()
		} else if dbRemove != "" {
			return handleRemoveDB()
		} else {
			return handleDBInfo(args)
		}
	},
}

func init() {
	dbCmd.Flags().BoolVarP(&dbList, "list", "l", false, "list available databases")
	dbCmd.Flags().BoolVar(&dbInit, "init", false, "initialize a new database")
	dbCmd.Flags().StringVar(&dbCreate, "create", "", "create a new database")
	dbCmd.Flags().StringVar(&dbRemove, "remove", "", "remove a database")
	rootCmd.AddCommand(dbCmd)
}
