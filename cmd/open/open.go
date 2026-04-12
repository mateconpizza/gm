package open

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "open [query]",
		Aliases: []string{"o"},
		Short:   "open in browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
			k := kb.NewKeymap("records snapshot open")
			k.Bind = cfg.Menu.DefaultKeymaps.Open.Bind
			k.Desc = "snapshot"
			k.Enabled = true

			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithBorderLabel(" "+config.AppName+" "),
				menu.WithHeaderLabel("open in default browser"),
				menu.WithHeaderBorder(menu.BorderRounded),
				menu.WithPreviewBorder(menu.BorderRounded),
				menu.WithHeaderFirst(),
				menu.WithKeybinds(k),
			)

			return base.RunWithBookmarks(cmd, args, m, handler.Open)
		},
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagMenu(c, cfg)
	base.FlagsFilter(c, cfg)

	return c
}
