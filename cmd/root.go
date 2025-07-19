package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/files"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/pkg/db"
)

// SkipDBCheckAnnotation is used in subcmds declarations to skip the database
// existence check.
var SkipDBCheckAnnotation = map[string]string{"skip-db-check": "true"}

var databaseChecked bool = false

func initRootFlags(cmd *cobra.Command) {
	cfg := config.App
	// global
	pf := cmd.PersistentFlags()
	pf.StringVarP(&cfg.DBName, "name", "n", config.MainDBName, "database name")
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
	cobra.EnableCommandSorting = false
}

// Root represents the base command when called without any subcommands.
var Root = &cobra.Command{
	Use:               config.App.Cmd,
	Short:             config.App.Info.Title,
	Long:              config.App.Info.Desc,
	Version:           prettyVersion(),
	Args:              cobra.MinimumNArgs(0),
	SilenceUsage:      true,
	PersistentPreRunE: RequireDatabase,
	RunE:              recordsCmdFunc,
}

func Execute() {
	if err := Root.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}

// RequireDatabase checks if the database exists.
func RequireDatabase(cmd *cobra.Command, args []string) error {
	if cmd.HasParent() {
		slog.Debug("assert db exists", "command", cmd.Name(), "parent", cmd.Parent().Name())
	} else {
		slog.Debug("assert db exists", "command", cmd.Name())
	}

	for c := cmd; c != nil; c = c.Parent() {
		if v, ok := c.Annotations["skip-db-check"]; ok && v == "true" {
			slog.Debug("skipping db check for", "command", c.Name())
			return nil
		}
	}

	if databaseChecked {
		return nil
	}

	if files.Exists(config.App.DBPath) {
		databaseChecked = true
		return nil
	}

	if err := handler.CheckDBLocked(config.App.DBPath); err != nil {
		return err
	}

	i := color.BrightYellow(config.App.Cmd, "init").Italic()
	if config.App.DBName == config.MainDBName {
		return fmt.Errorf("%w: use '%s' to initialize", db.ErrDBMainNotFound, i)
	}

	return fmt.Errorf("%w %q: use '%s' to initialize", db.ErrDBNotFound, config.App.DBName, i)
}
