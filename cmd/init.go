/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/color"
	"gomarks/pkg/config"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new bookmarks database and table",
	RunE: func(cmd *cobra.Command, _ []string) error {
		r, err := database.GetDB()
		if err == nil {
			return fmt.Errorf("%w", errs.ErrDBAlreadyInitialized)
		}

		if !errors.Is(err, errs.ErrDBNotFound) {
			return fmt.Errorf("initializing database: %w", err)
		}

		err = util.SetupProjectPaths()
		if err != nil {
			return fmt.Errorf("creating home: %w", err)
		}

		if err = r.InitDB(); err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}

		printSummary()

		bs, err := r.GetRecordsAll(config.DB.Table.Main)
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
	fmt.Printf("%s v%s:\n", config.App.Name, config.App.Info.Version)
	fmt.Printf("  + app home created at: %s\n", color.Colorize(config.Path.Home, color.Yellow))
	fmt.Printf("  + database '%s' initialized\n", color.Colorize(config.DB.Name, color.Green))
	fmt.Printf("  + %s bookmark created\n\n", color.Colorize("initial", color.Purple))
}

func init() {
	rootCmd.AddCommand(initCmd)
}
