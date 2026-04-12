package rm

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm [query]",
		Aliases: []string{"remove"},
		Short:   "remove bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			m := handler.MenuSimple[bookmark.Bookmark](cfg)
			return base.RunWithBookmarks(cmd, args, m, handler.Remove)
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

	return c
}
