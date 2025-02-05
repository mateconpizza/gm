package cmd

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
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
	WithColor string

	Force   bool
	Status  bool
	Verbose bool
)

func initConfig() {
	// set logging level
	handler.LoggingLevel(&Verbose)
	// set force
	handler.Force(&Force)
	// enable color
	config.App.Color = WithColor != "never" && !terminal.IsPiped()
	menu.WithColor(&config.App.Color)
	// set terminal defaults
	terminal.NoColor(&config.App.Color)
	terminal.LoadMaxWidth()
	// enable color output
	color.Enable(&config.App.Color)
	// load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		sys.ErrAndExit(err)
	}
	// set app/database settings/paths
	config.App.Path.Data = dataHomePath                            // Home
	config.App.Path.Backup = filepath.Join(dataHomePath, "backup") // Backups
	p := config.App.Path
	Cfg = repo.NewSQLiteCfg()
	Cfg.SetName(DBName).SetPath(p.Data).SetBackupPath(p.Backup)
	Cfg.Backup.SetLimit(backupGetLimit())
}

// init sets the config for the root command.
func init() {
	cobra.OnInitialize(initConfig)
	// global
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&DBName, "name", "n", config.DB.Name, "database name")
	pf.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	pf.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	pf.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	_ = pf.MarkHidden("help")
	// cmd settings
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceErrors = true
	rootCmd.DisableSuggestions = true
	rootCmd.SuggestionsMinimumDistance = 1
}

// isSubCmdCalled returns true if the subcommand was called.
func isSubCmdCalled(cmd *cobra.Command, cmdName string) bool {
	p := cmd.Parent()
	if p == nil {
		return false
	}
	for _, subCmd := range p.Commands() {
		if subCmd.CalledAs() == cmdName {
			return true
		}
	}

	return false
}
