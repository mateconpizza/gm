/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"errors"
	"fmt"
	"os"

	"gomarks/pkg/actions"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	"gomarks/pkg/errs"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var (
	Menu      *menu.Menu
	Verbose   bool
	Bookmarks *bookmark.Slice
	// idFlag  int
)

func checkInitDB(_ *cobra.Command, _ []string) error {
	_, err := getDB()
	if err != nil {
		if errors.Is(err, errs.ErrDBNotFound) {
			return fmt.Errorf("%w: use 'init' to initialise a new database", errs.ErrDBNotFound)
		}
		return fmt.Errorf("%w", err)
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:          "gomarks",
	Short:        "Gomarks is a bookmark manager for your terminal",
	Long:         "Gomarks is a bookmark manager for your terminal",
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	PreRunE:      checkInitDB,
	RunE: func(_ *cobra.Command, args []string) error {
		query := handleQuery(args)

		r, _ := getDB()

		bs, err := r.GetRecordsByQuery(constants.DBMainTableName, query)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if Menu != nil {
			var b bookmark.Bookmark
			b, err = display.SelectBookmark(Menu, bs)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			bs = &bookmark.Slice{b}
		}

		err = actions.HandleFormat("pretty", bs)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		util.CopyToClipboard((*bs)[0].URL)

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	var copyFlag bool
	var menuFlag string
	var openFlag bool
	var queryFlag string

	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVarP(&queryFlag, "query", "", "", "query to filter bookmarks")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVarP(&copyFlag, "copy", "c", true, "copy to system clipboard")
	rootCmd.PersistentFlags().BoolVarP(&openFlag, "open", "o", false, "open in default browser")
	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu | rofi]")
	// rootCmd.PersistentFlags().IntVarP(&idFlag, "id", "", 0, "select bookmark by id")
}

func initConfig() {
	util.SetLogLevel(&Verbose)
	Menu = handleMenu()
}

func getDB() (*database.SQLiteRepository, error) {
	r, err := database.GetDB()
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	return r, nil
}

func handleMenu() *menu.Menu {
	menuName, err := rootCmd.Flags().GetString("menu")
	if err != nil {
		fmt.Println("err on getting menu:", err)
	}

	if menuName == "" {
		return nil
	}

	return menu.New(menuName)
}

func handleQuery(args []string) string {
	var query string
	if len(args) == 0 {
		query = ""
	} else {
		query = args[0]
	}
	return query
}
