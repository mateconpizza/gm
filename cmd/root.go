/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"os"

	"gomarks/pkg/database"

	"github.com/spf13/cobra"
)

var Verbose bool

var rootCmd = &cobra.Command{
	Use:   "gomarks",
	Short: "Gomarks is a bookmark manager for your terminal",
	Long:  "Gomarks is a bookmark manager for your terminal",
}

// func isVerbose(cmd *cobra.Command) bool {
// 	verbose, err := cmd.PersistentFlags().GetBool("verbose")
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
	var query string
	var id int

	cobra.OnInitialize(initConfig)

	rootCmd.Flags().StringVarP(&query, "query", "", "", "query to filter bookmarks")
	rootCmd.PersistentFlags().IntVarP(&id, "id", "", 0, "select bookmark by id")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
}

func initConfig() {
	query, err := rootCmd.Flags().GetString("query")
	if err != nil {
		fmt.Println("err:", err)
		return
	}

	if query != "" {
		fmt.Println("NewQuery::::", query)
	}
}

func getDB() *database.SQLiteRepository {
	r := database.GetDB()
	return r
}
