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
			k := menu.NewKeymap()
			k = k.WithSilentAction(kb.BaseCmd("archive open") + " {1}")
			k.Bind = "ctrl-o"
			k.Desc = "open-snapshot"
			k.Enabled = true

			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeaderLabel(" open in browser "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
				menu.WithKeybinds(k),
			)

			return base.Execute(cmd, args, m, handler.Open)
		},
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagMenu(c, cfg)
	base.FlagsFilter(c, cfg)

	return c
}
