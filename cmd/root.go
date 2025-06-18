package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
)

// CLI Flags.
var (
	Copy        bool     // Copy URL into clipboard
	Open        bool     // Open URL in default browser
	Tags        []string // Tags list to filter bookmarks
	QR          bool     // QR code generator
	Menu        bool     // Menu mode
	Edit        bool     // Edit mode
	Head        int      // Head limit
	Remove      bool     // Remove bookmarks
	Tail        int      // Tail limit
	Field       string   // Field to print
	JSON        bool     // JSON output
	Oneline     bool     // Oneline output
	Multiline   bool     // Multiline output
	WithColor   string   // WithColor enable color output
	Force       bool     // Force action
	Status      bool     // Status checks URLs status code
	VerboseFlag int      // Verbose flag
)

func initRootFlags(cmd *cobra.Command) {
	// global
	pf := cmd.PersistentFlags()
	pf.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	pf.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	pf.CountVarP(&VerboseFlag, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
	pf.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	_ = pf.MarkHidden("help")

	initRecordFlags(cmd)

	// cmd settings
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.SilenceErrors = true
	cmd.DisableSuggestions = true
	cmd.SuggestionsMinimumDistance = 1
}

// Root represents the base command when called without any subcommands.
var Root = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Info.Title,
	Long:         config.App.Info.Desc,
	Version:      prettyVersion(),
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return handler.AssertDatabaseExists(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := recordsCmd.PersistentPreRunE(cmd, args); err != nil {
			return fmt.Errorf("%w", err)
		}

		return recordsCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := Root.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}
