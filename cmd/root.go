package cmd

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/encryptor"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/slice"
	"github.com/haaag/gm/internal/sys"
)

type (
	Bookmark = bookmark.Bookmark
	Slice    = slice.Slice[Bookmark]
	Repo     = repo.SQLiteRepository
)

var (
	// SQLiteCfg holds the configuration for the database and backups.
	Cfg *repo.SQLiteCfg

	// Main database name.
	DBName string
)

// handleData processes records based on user input and filtering criteria.
func handleData(m *menu.Menu[Bookmark], r *Repo, args []string) (*Slice, error) {
	bs := slice.New[Bookmark]()
	if err := handler.Records(r, bs, args); err != nil {
		return nil, fmt.Errorf("%w", err)
	}
	// filter by Tag
	if len(Tags) > 0 {
		if err := handler.ByTags(r, Tags, bs); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}
	// filter by head and tail
	if Head > 0 || Tail > 0 {
		if err := handler.ByHeadAndTail(bs, Head, Tail); err != nil {
			return nil, fmt.Errorf("%w", err)
		}
	}
	// select with fzf-menu
	if Menu || Multiline {
		items, err := handler.Selection(
			m,
			*bs.Items(),
			handler.FzfFormatter(Multiline, config.App.Colorscheme),
		)
		if err != nil {
			return nil, fmt.Errorf("%w", err)
		}
		bs.Set(&items)
	}

	return bs, nil
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Info.Title,
	Long:         config.App.Info.Desc,
	Version:      prettyVersion(),
	Args:         cobra.MinimumNArgs(0),
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		// ignore if one of this subcommands was called.
		subcmds := []string{"init", "new", "version", "lock", "unlock"}
		if isSubCmdCalled(cmd, subcmds...) {
			return nil
		}
		p := filepath.Join(config.App.Path.Data, config.App.DBName)
		if err := encryptor.IsEncrypted(p); err != nil {
			if errors.Is(err, encryptor.ErrFileEncrypted) {
				return repo.ErrDBDecryptFirst
			}

			return fmt.Errorf("%w", err)
		}

		return handler.ValidateDB(cmd, Cfg)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return recordsCmd.RunE(cmd, args)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		sys.ErrAndExit(err)
	}
}
