package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/pkg/app"
	"github.com/haaag/gm/pkg/bookmark"
	"github.com/haaag/gm/pkg/editor"
	"github.com/haaag/gm/pkg/format"
	"github.com/haaag/gm/pkg/repo"
	"github.com/haaag/gm/pkg/terminal"
)

// TODO)):
// - [ ] use io.Reader for read in chunks
//  - [ ] modify functions with []byte or json.Marshal
// - [ ] remove verbose settings, better use a library for logging
// ## Editor
// - [X] create a pkg named editor
// ## Terminal
// - [X] create a pkg named terminal

type (
	Bookmark   = bookmark.Bookmark
	Slice      = bookmark.Slice[Bookmark]
	Repository = repo.SQLiteRepository
)

var (
	// FIX: Remove this Global Exit
	Exit bool

	// Main database name
	DBName string

	// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set
	textEditors = []string{"vim", "nvim", "nano", "emacs", "helix"}

	// App is the configuration and info for the application
	App = app.New()

	// SQLiteCfg holds the configuration for the database and backups
	Cfg *repo.SQLiteConfig
)

var rootCmd = &cobra.Command{
	Use:          App.Cmd,
	Short:        App.Info.Title,
	Long:         App.Info.Desc,
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !dbExistsAndInit(Cfg.GetHome(), DBName) && !DBInit {
			init := format.Color("--init").Yellow().Bold()
			return fmt.Errorf("%w: use %s", repo.ErrDBNotFound, init)
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		if DBInit {
			return handleDBInit()
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

		bs := bookmark.NewSlice[Bookmark]()
		if err := handleListAndEdit(r, bs, args); err != nil {
			return err
		}

		return handleOutput(bs)
	},
}

func initConfig() {
	// Set logging level
	setLoggingLevel(&Verbose)

	// Set terminal defaults and color output
	terminal.SetIsPiped(terminal.IsPiped())
	terminal.SetColor(WithColor != "never" && !Json && !terminal.Piped)
	terminal.LoadMaxWidth()

	// Load editor
	if err := editor.Load(&App.Env.Editor, &textEditors); err != nil {
		logErrAndExit(err)
	}

	// Load App home path
	if err := app.LoadHome(App); err != nil {
		logErrAndExit(err)
	}

	// Set database settings
	Cfg = repo.NewSQLiteCfg()
	Cfg.SetName(ensureDbSuffix(DBName))
	Cfg.SetHome(App.GetHome())
	Cfg.Backup.SetMax(App.Env.BackupMax)

	// Create dirs for the app
	if err := app.CreateHome(App, Cfg.Backup.GetHome()); err != nil {
		logErrAndExit(err)
	}
}

func handleListAndEdit(r *Repository, bs *Slice, args []string) error {
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
	return handleEdition(r, bs)
}

func handleOutput(bs *Slice) error {
	if err := handleOneline(bs); err != nil {
		return err
	}
	if err := handleJsonFormat(bs); err != nil {
		return err
	}
	if err := handleByField(bs); err != nil {
		return err
	}

	if bs.Len() == 0 {
		return repo.ErrRecordNotFound
	}

	if err := handleCopyOpen(bs); err != nil {
		return err
	}

	return handleFormat(bs)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logErrAndExit(err)
	}
}
