package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/terminal"
	"github.com/haaag/gm/pkg/slice"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
)

// TODO)):
// - [x] Extract `restore|deleted` logic to subcommand `restore`.
// - [x] Extract `init` logic to subcommand `init`.
// WARN:
// - [ ] Simplify `root.go`

var (
	// SQLiteCfg holds the configuration for the database and backups.
	Cfg *repo.SQLiteConfig

	// Main database name.
	DBName string

	// FIX: Remove this Global Exit.
	Exit bool

	// Fallback text editors if $EDITOR || $GOMARKS_EDITOR var is not set.
	// FIX: Remove this fallback.
	textEditors = []string{"vim", "nvim", "nano", "emacs"}
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Info.Title,
	Long:         config.App.Info.Desc,
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return verifyDatabase(Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		r, err := repo.New(Cfg)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()

		terminal.ReadPipedInput(&args)

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

func handleListAndEdit(r *repo.SQLiteRepository, bs *Slice, args []string) error {
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
	if err := handleMenu(bs); err != nil {
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
	/* if err := handleOneline(bs); err != nil {
		return err
	} */
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

	return handlePrintOut(bs)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		logErrAndExit(err)
	}
}
