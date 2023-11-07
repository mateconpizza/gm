/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"os"

	"gomarks/pkg/actions"
	"gomarks/pkg/database"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var Verbose bool

var rootCmd = &cobra.Command{
	Use:   "gomarks",
	Short: "Gomarks is a bookmark manager for your terminal",
	Long:  "Gomarks is a bookmark manager for your terminal",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		fmt.Println("LEN ARGS::::", len(args))
		var query string

		if len(args) == 0 {
			query = ""
		} else {
			query = args[0]
		}

		r := getDB()

		bs, err := r.GetRecordsByQuery(query, "bookmarks")
		if err != nil {
			return
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
	var menu string
	var query string
	var copyFlag bool
	var openFlag bool

	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVarP(&query, "query", "", "", "query to filter bookmarks")
	rootCmd.PersistentFlags().StringVarP(&menu, "menu", "m", "", "menu mode [dmenu | rofi]")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	rootCmd.PersistentFlags().
		BoolVarP(&copyFlag, "copy", "c", true, "copy to system clipboar (default)")
	rootCmd.PersistentFlags().
		BoolVarP(&openFlag, "open", "o", false, "open in default browser")
}

func initConfig() {
	util.SetLogLevel(&Verbose)
}

func getDB() *database.SQLiteRepository {
	r := database.GetDB()
	return r
}
