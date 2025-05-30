package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
)

var _ = `
  database    Database management
  records     Records management
`

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
	// local
	f := cmd.Flags()
	// prints
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	f.BoolVarP(&Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	f.StringVarP(&Field, "field", "f", "", "output by field [id,1|url,2|title,3|tags,4]")
	// actions
	f.BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	f.BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&QR, "qr", "q", false, "generate qr-code")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&Tags, "tag", "t", nil, "list by tag")
	// experimental
	f.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&Edit, "edit", "e", false, "edit with preferred text editor")
	f.BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	// modifiers
	f.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
	// cmd settings
	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.SilenceErrors = true
	cmd.DisableSuggestions = true
	cmd.SuggestionsMinimumDistance = 1
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Info.Title,
	Long:         config.App.Info.Desc,
	Version:      prettyVersion(),
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return handler.AssertDefaultDatabaseExists()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := recordsCmd.PersistentPreRunE(cmd, args); err != nil {
			return fmt.Errorf("%w", err)
		}

		return recordsCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}
