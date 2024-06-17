package cmd

import (
	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
)

var (
	Copy bool
	List bool
	Open bool
	Tags string

	Add    bool
	Edit   bool
	Head   int
	Remove bool
	Tail   int

	Field     string
	Json      bool
	Oneline   bool
	Prettify  bool
	WithColor string

	DBInit  bool
	Force   bool
	Status  bool
	Verbose bool
	Version bool
)

func init() {
	cobra.OnInitialize(initConfig)

	// Global
	rootCmd.PersistentFlags().BoolVar(&DBInit, "init", false, "initialize a database")
	rootCmd.PersistentFlags().StringVarP(&DBName, "name", "n", app.DefaultDBName, "database name")
	rootCmd.PersistentFlags().BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVarP(&Json, "json", "j", false, "print data in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&Prettify, "pretty", "p", false, "print data in pretty format")
	rootCmd.PersistentFlags().StringVar(&WithColor, "color", "always", "print with pretty colors [always|never]")
	rootCmd.MarkFlagsMutuallyExclusive("json", "pretty")

	// Actions
	rootCmd.Flags().BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	rootCmd.Flags().BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	rootCmd.Flags().BoolVarP(&List, "list", "l", false, "list all bookmarks")
	rootCmd.Flags().StringVarP(&Tags, "tag", "t", "", "filter bookmarks by tag")
	rootCmd.Flags().BoolVarP(&Add, "add", "a", false, "add a new bookmark")

	// Experimental
	rootCmd.Flags().BoolVarP(&Edit, "edit", "e", false, "edit mode")
	rootCmd.Flags().BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	rootCmd.Flags().BoolVar(&Oneline, "oneline", false, "output formatted oneline data")
	rootCmd.Flags().BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	rootCmd.Flags().StringVarP(&Field, "field", "F", "", "prints by field [id|url|title|tags]")

	// Modifiers
	rootCmd.Flags().IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	rootCmd.Flags().IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")

	// Others
	rootCmd.Flags().BoolVarP(&Version, "version", "v", false, "print version info")
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceErrors = true
}
