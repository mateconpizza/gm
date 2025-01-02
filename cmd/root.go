package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys/terminal"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
)

// TODO:
// - [x] Extract `restore|deleted` logic to subcommand `restore`.
// - [x] Extract `init` logic to subcommand `init`.

var (
	// SQLiteCfg holds the configuration for the database and backups.
	Cfg *repo.SQLiteConfig

	// Main database name.
	DBName string

	// FIX: Find a better way to handle exit.
	Exit bool

	// subCommandCalled is used to check if the subcommand was called, to modify
	// some aspects of the program flow, and menu options.
	subCommandCalled bool
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
		if err := handleRecords(r, bs, args); err != nil {
			return err
		}
		if err := handleAction(r, bs); err != nil {
			return err
		}
		if err := handleCheckStatus(bs); err != nil {
			return err
		}

		return handleOutput(bs)
	},
}

// handleRecords retrieve records.
func handleRecords(r *repo.SQLiteRepository, bs *Slice, args []string) error {
	if err := handleIDsFromArgs(r, bs, args); err != nil {
		return err
	}
	if err := handleByQuery(r, bs, args); err != nil {
		return err
	}
	if err := handleByTags(r, bs); err != nil {
		return err
	}

	if bs.Empty() && len(args) == 0 {
		// get all records
		if err := r.Records(r.Cfg.TableMain, bs); err != nil {
			return fmt.Errorf("getting records: %w", err)
		}

		Frame = true
	}
	if err := handleHeadAndTail(bs); err != nil {
		return err
	}

	return handleMenu(bs)
}

func handleAction(r *repo.SQLiteRepository, bs *Slice) error {
	if err := handleRemove(r, bs); err != nil {
		return err
	}

	defer r.Close()

	return handleEdition(r, bs)
}

func handleOutput(bs *Slice) error {
	if err := handleJSONFormat(bs); err != nil {
		return err
	}
	if err := handleOneline(bs); err != nil {
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
