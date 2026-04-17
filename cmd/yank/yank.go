package yank

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "yank [query]",
		Aliases: []string{"copy", "c", "y"},
		Short:   "copy URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" yank URL "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				var sb strings.Builder
				for i := range bs {
					sb.WriteString(bs[i].URL + "\n")
				}

				return sys.CopyClipboard(sb.String())
			})
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	cmdutil.HideFlag(c, "help")
	cmdutil.FlagsFilter(c, cfg)

	return c
}
