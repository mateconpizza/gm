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

// rootCmd represents the base command when called without any subcommands
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
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
	rootCmd.PersistentFlags().
		StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
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

	_, err = rootCmd.Flags().GetInt("id")
	if err != nil {
		fmt.Println("err:", err)
		return
	}
}

func initDB() *database.SQLiteRepository {
	db := database.GetDB()
	return db
}
