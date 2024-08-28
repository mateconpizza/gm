package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format/color"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/slice"
	"github.com/haaag/gm/pkg/terminal"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
	Repo     = repo.SQLiteRepository
)

var (
	// FIX: Remove this Global Exit.
	Exit bool

	// Main database name.
	DBName string

	// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set.
	// FIX: Remove this fallback.
	textEditors = []string{"vim", "nvim", "nano", "emacs", "helix"}

	// App is the config with default values for the app.
	App = app.New()

	// SQLiteCfg holds the configuration for the database and backups.
	Cfg *repo.SQLiteConfig
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          App.Cmd,
	Short:        App.Info.Title,
	Long:         App.Info.Desc,
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !dbExistsAndInit(Cfg.Path, DBName) && !DBInit {
			init := color.Yellow("--init").Bold().Italic()
			return fmt.Errorf("%w: use %s", repo.ErrDBNotFound, init)
		}

		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		if Version {
			fmt.Print(app.PrettyVersion(Prettify))
			return nil
		}

		if DBInit {
			return handleDBInit()
		}

		// FIX: find better way
		if Deleted {
			Cfg.TableMain = Cfg.TableDeleted
		}

		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

		if Add {
			return handleAdd(r, args)
		}

		bs := slice.New[Bookmark]()
		if err := handleListAndEdit(r, bs, args); err != nil {
			return err
		}

		if bs.Len() == 0 {
			return repo.ErrRecordNoMatch
		}

		return handleOutput(bs)
	},
}

func initConfig() {
	// Set logging level
	setLoggingLevel(&Verbose)

	// Set terminal defaults and color output
	terminal.SetIsPiped(terminal.IsPiped())
	terminal.SetColor(WithColor != "never" && !JSON && !terminal.Piped)
	terminal.LoadMaxWidth()
	color.Enable(&terminal.Color)

	// Load editor
	if err := editor.Load(&App.Env.Editor, &textEditors); err != nil {
		logErrAndExit(err)
	}

	// Load App home path
	if err := app.LoadPath(App); err != nil {
		logErrAndExit(err)
	}

	// Set database settings
	Cfg = repo.NewSQLiteCfg()
	Cfg.SetDefaults(App.Path, DBName, App.Env.BackupMax)

	// Create dirs for the app
	if err := app.CreatePaths(App, Cfg.BackupPath); err != nil {
		logErrAndExit(err)
	}
}

func handleListAndEdit(r *Repo, bs *Slice, args []string) error {
	if err := handleListAll(r, bs); err != nil {
		return err
	}
	if err := handleByTags(r, bs); err != nil {
		return err
	}
	if err := handleIDsFromArgs(r, bs, args); err != nil {
		return err
	}
	if err := handleByQuery(r, bs, args); err != nil {
		return err
	}
	if err := handleHeadAndTail(bs); err != nil {
		return err
	}
	if err := handleCheckStatus(bs); err != nil {
		return err
	}
	if err := handleRemove(r, bs); err != nil {
		return err
	}
	if err := handleRestore(r, bs); err != nil {
		return err
	}

	return handleEdition(r, bs)
}

func handleOutput(bs *Slice) error {
	if err := handleOneline(bs); err != nil {
		return err
	}
	if err := handleJSONFormat(bs); err != nil {
		return err
	}
	if err := handleByField(bs); err != nil {
		return err
	}
	if err := handleQR(bs); err != nil {
		return err
	}
	if err := handleCopyOpen(bs); err != nil {
		return err
	}

	if err := handleFrame(bs); err != nil {
		return err
	}

	return handleFormat(bs)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logErrAndExit(err)
	}
}
