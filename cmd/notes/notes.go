package notes

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
		Use:     "notes [query]",
		Aliases: []string{"n"},
		Short:   "view/edit notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Flags.Notes = true
			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)

			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeaderFirst(),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(cfg.Menu.DefaultKeymaps.Edit)),
			)
			return base.RunWithBookmarks(cmd, args, m, handler.Notes)
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

	c.AddCommand(newEditNotesCmd(cfg))

	return c
}

func newEditNotesCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [query]",
		Short: "edit notes with text editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.Flags.Notes = true
			cfg.Flags.Edit = true

			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeaderFirst(),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(cfg.Menu.DefaultKeymaps.Edit)),
			)

			return base.RunWithBookmarks(cmd, args, m, handler.Edit)
		},
	}

	base.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

	return c
}
