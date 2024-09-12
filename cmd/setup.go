package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/internal/util/files"
)

var (
	Copy bool
	List bool
	Open bool
	Tags []string
	QR   bool

	Menu   bool
	Edit   bool
	Head   int
	Remove bool
	Tail   int

	Field     string
	JSON      bool
	Oneline   bool
	Multiline bool
	Frame     bool
	WithColor string

	Force   bool
	Status  bool
	Verbose bool
)

func initConfig() {
	// Set logging level
	setLoggingLevel(&Verbose)

	// Set terminal defaults
	terminal.SetColor(WithColor != "never" && !terminal.IsPiped())
	terminal.LoadMaxWidth()

	// Enable color output
	color.EnableANSI(&terminal.Color)

	// Load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		logErrAndExit(err)
	}
	config.App.Path.Data = dataHomePath
	config.App.Path.Backup = filepath.Join(dataHomePath, "backup")

	// Set database settings/paths
	Cfg = repo.NewSQLiteCfg(dataHomePath)
	Cfg.SetName(DBName)
	Cfg.Backup.SetLimit(getMaxBackup())

	// Create paths for the application.
	if err := files.MkdirAll(config.App.Path.Backup); err != nil {
		logErrAndExit(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global
	rootCmd.PersistentFlags().
		StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	rootCmd.PersistentFlags().
		BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	rootCmd.PersistentFlags().BoolVar(&Verbose, "verbose", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVarP(&JSON, "json", "j", false, "print data in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&Frame, "frame", "f", false, "print data in framed format")
	rootCmd.PersistentFlags().
		StringVar(&WithColor, "color", "never", "print data in pretty colors [always|never]")
	rootCmd.MarkFlagsMutuallyExclusive("json", "frame")

	// Actions
	rootCmd.Flags().BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	rootCmd.Flags().BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	rootCmd.Flags().BoolVarP(&List, "list", "l", false, "list all bookmarks")
	rootCmd.Flags().StringSliceVarP(&Tags, "tags", "t", nil, "bookmarks by tag")
	rootCmd.Flags().BoolVar(&QR, "qr", false, "generate qr-code")

	// Experimental
	rootCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	rootCmd.Flags().BoolVarP(&Edit, "edit", "e", false, "edit mode")
	rootCmd.Flags().BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	rootCmd.Flags().BoolVar(&Oneline, "oneline", false, "output formatted oneline data")
	rootCmd.Flags().BoolVar(&Multiline, "multiline", false, "output formatted multiline data")
	rootCmd.Flags().BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	rootCmd.Flags().StringVarP(&Field, "field", "F", "", "prints by field [id|url|title|tags]")

	// Modifiers
	rootCmd.Flags().IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	rootCmd.Flags().IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")

	// Others
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceErrors = true
	rootCmd.DisableSuggestions = true
	rootCmd.SuggestionsMinimumDistance = 1
}

// verifyDatabase verifies if the database exists.
func verifyDatabase(c *repo.SQLiteConfig) error {
	db := files.EnsureExtension(DBName, ".db")
	i := color.BrightYellow(config.App.Cmd, "init").Bold().Italic()

	if err := c.Exists(); err != nil {
		return fmt.Errorf("%w: %s to initialize '%s'", repo.ErrDBNotFound, i, db)
	}

	return nil
}
