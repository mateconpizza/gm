package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys/terminal"
)

var (
	Copy bool
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
	Frame     bool // FIX: Remove
	WithColor string

	Force   bool
	Status  bool
	Verbose bool
)

func initConfig() {
	// Set logging level
	setLoggingLevel(&Verbose)

	// Set color eanble
	config.App.Color = WithColor != "never" && !terminal.IsPiped()

	// Set terminal defaults
	terminal.NoColor(&config.App.Color)
	terminal.LoadMaxWidth()

	// Enable color output
	color.Enable(&config.App.Color)

	// Load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		logErrAndExit(err)
	}
	config.App.Path.Data = dataHomePath                            // Home
	config.App.Path.Backup = filepath.Join(dataHomePath, "backup") // Backups

	// Set database settings/paths
	Cfg = repo.NewSQLiteCfg(dataHomePath)
	Cfg.SetName(DBName)
	Cfg.Backup.SetLimit(backupGetLimit())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global
	rootCmd.PersistentFlags().
		StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	rootCmd.PersistentFlags().
		BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")

	// Prints
	rootCmd.Flags().BoolVar(&JSON, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().
		StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	rootCmd.Flags().
		BoolVarP(&Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	rootCmd.Flags().
		BoolVarP(&Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	rootCmd.Flags().StringVarP(&Field, "field", "f", "", "output by field [id|url|title|tags]")

	// Actions
	rootCmd.Flags().BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	rootCmd.Flags().BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	rootCmd.Flags().StringSliceVarP(&Tags, "tag", "t", nil, "list by tag")
	rootCmd.Flags().BoolVarP(&QR, "qr", "q", false, "generate qr-code")

	// Experimental
	rootCmd.Flags().BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	rootCmd.Flags().BoolVarP(&Edit, "edit", "e", false, "edit with preferred text editor")
	rootCmd.Flags().BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	rootCmd.Flags().BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")

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
	if c.Exists() {
		return nil
	}

	s := color.BrightYellow(config.App.Cmd, "init").Italic()
	init := fmt.Errorf("%w: use '%s' to initialize", repo.ErrDBNotFound, s)
	databases, err := repo.Databases(c)
	if err != nil {
		return init
	}

	// find with no|other extension
	databases.ForEach(func(r *repo.SQLiteRepository) {
		s := strings.TrimSuffix(r.Cfg.Name, filepath.Ext(r.Cfg.Name))
		if s == DBName {
			c.Name = r.Cfg.Name
		}
	})

	if c.Exists() {
		return nil
	}

	return init
}
