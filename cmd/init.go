/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/app"
	"gomarks/pkg/database"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new bookmarks database",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := database.GetDB()
		if err == nil {
			return fmt.Errorf("%w", database.ErrDBAlreadyInitialized)
		}

		if !errors.Is(err, database.ErrDBNotFound) {
			return fmt.Errorf("initializing database: %w", err)
		}

		if err = util.SetupProjectPaths(); err != nil {
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

		if err := handleFormat(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func printSummary() {
	fmt.Printf("%s v%s:\n", config.App.Name, config.App.Version)
	fmt.Printf("  + app home created at: %s\n", format.Text(config.App.Path.Home).Yellow().Bold())
	fmt.Printf("  + database '%s' initialized\n", format.Text(config.DB.Name).Green())
	fmt.Printf("  + %s bookmark created\n\n", format.Text("initial").Purple())
}

func init() {
	rootCmd.AddCommand(initCmd)
}
