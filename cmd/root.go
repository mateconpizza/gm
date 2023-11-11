/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"log"
	"os"

	"gomarks/pkg/actions"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/constants"
	"gomarks/pkg/display"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var (
	Menu      *menu.Menu
	Verbose   bool
	Bookmarks *bookmark.Slice
)

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
			var b *bookmark.Bookmark
			b, err = display.SelectBookmark(Menu, bs)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			bs = &bookmark.Slice{*b} // FIX: after selecting from query, work with a single bookmark
		}

		if err := actions.HandleFormat("pretty", bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		util.CopyToClipboard(
			(*bs)[0].URL,
		) // FIX: after selecting from query, work with a single bookmark

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
	var queryFlag string

	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVarP(&queryFlag, "query", "", "", "query to filter bookmarks")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVarP(&copyFlag, "copy", "c", true, "copy to system clipboard")
	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu | rofi]")
}

func initConfig() {
	var err error
	util.SetLogLevel(&Verbose)
	Menu, err = handleMenu()
	if err != nil {
		log.Fatal(err)
	}
}
