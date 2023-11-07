/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"os"

	"gomarks/pkg/actions"
	"gomarks/pkg/bookmark"
	"gomarks/pkg/database"
	"gomarks/pkg/display"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var (
	Menu    *menu.Menu
	Verbose bool
	idFlag  int
)

var rootCmd = &cobra.Command{
	Use:   "gomarks",
	Short: "Gomarks is a bookmark manager for your terminal",
	Long:  "Gomarks is a bookmark manager for your terminal",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		query := handleQuery(args)
		r := getDB()

		bs, err := r.GetRecordsByQuery(query, "bookmarks")
		if err != nil {
			return
		}

		if Menu != nil {
			var b bookmark.Bookmark
			b, err = display.SelectBookmark(Menu, bs)
			if err != nil {
				fmt.Println("err on menu:", err)
			}
			bs = &bookmark.Slice{b}
		}

		err = actions.HandleFormat("pretty", bs)
		if err != nil {
			return
		}

		util.CopyToClipboard((*bs)[0].URL)
	},
}

// func isVerbose(cmd *cobra.Command) bool {
// 	verbose, err := cmd.PersistentFlags().GetBool("Verbose")
// 	if err != nil {
// 		panic(err)
// 	}
// 	return verbose
// }

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
	rootCmd.PersistentFlags().IntVarP(&idFlag, "id", "", 0, "select bookmark by id")
	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu | rofi]")
}

func initConfig() {
	util.SetLogLevel(&Verbose)
	Menu = handleMenu()
	handleID()
}

func getDB() *database.SQLiteRepository {
	r := database.GetDB()
	return r
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

func handleID() {
	id, err := rootCmd.Flags().GetInt("id")
	if err != nil {
		fmt.Println("err getting id:", err)
	}
	idFlag = id
}
