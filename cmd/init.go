// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"log"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"

	"github.com/spf13/cobra"
)

func printSummary() {
	fmt.Printf("%s v%s:\n\n", config.App.Name, config.App.Version)
	fmt.Printf("+ app home created at: %s\n", format.Text(config.App.Path.Home).Yellow().Bold())
	fmt.Printf("+ database '%s' initialized\n", format.Text(dbName).Green())
	fmt.Printf("+ %s bookmark created\n\n", format.Text("initial").Purple())
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new bookmarks database",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := bookmark.NewRepository(dbName)
		if err == nil {
			return fmt.Errorf("%w", bookmark.ErrDBAlreadyInitialized)
		}

		defer func() {
			if err := r.DB.Close(); err != nil {
				log.Printf("closing database: %v", err)
			}
		}()

		if !errors.Is(err, bookmark.ErrDBNotFound) {
			return fmt.Errorf("initializing database: %w", err)
		}

		if err = config.SetupProjectPaths(); err != nil {
			return fmt.Errorf("creating home: %w", err)
		}

		if err = r.Init(); err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}

		printSummary()

		bs, err := r.GetAll(config.DB.Table.Main)
		if err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		if err := handleFormat(bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

//
// func init() {
// 	rootCmd.AddCommand(initCmd)
// }
