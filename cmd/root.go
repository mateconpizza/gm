package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys"
)

func initRootFlags(cmd *cobra.Command) {
	cfg := config.App
	// global
	pf := cmd.PersistentFlags()
	pf.StringVarP(&cfg.DBName, "name", "n", config.DefaultDBName, "database name")
	pf.StringVar(&cfg.Flags.ColorStr, "color", "always", "output with pretty colors [always|never]")
	pf.CountVarP(&cfg.Flags.Verbose, "verbose", "v", "Increase verbosity (-v, -vv, -vvv)")
	pf.BoolVar(&cfg.Flags.Force, "force", false, "force action | don't ask confirmation")
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
