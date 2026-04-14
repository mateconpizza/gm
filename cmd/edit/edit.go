package edit

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "edit [query]",
		Aliases: []string{"e"},
		Short:   "edit bookmark",
		RunE: func(cmd *cobra.Command, args []string) error {
			// BUG: menu: current functionality exits the menu after editing a bookmark.
			// New functionality must keep menu after editing.

			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
			k := kb.NewKeymap("edit --format json")
			k.Bind = cfg.Menu.DefaultKeymaps.Edit.Bind
			k.Desc = "as-json"
			k.Enabled = true

			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" edition "),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
				menu.WithKeybinds(k),
			)

			var strategy editor.EditStrategy
			strategy = editor.BookmarkStrategy{}
			if cfg.Flags.Format == "j" || cfg.Flags.Format == "json" {
				strategy = editor.JSONStrategy{}
			}

			return cmdutil.Execute(cmd, args, m, handler.Edit(strategy))
		},
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	c.Flags().StringVarP(&cfg.Flags.Format, "format", "f", "",
		fmt.Sprintf("output format [%s]", strings.Join(printer.ValidFormats, "|")))

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)

	return c
}
