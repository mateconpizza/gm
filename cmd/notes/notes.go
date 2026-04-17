package notes

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/editor"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "notes [query]",
		Aliases: []string{"n"},
		Short:   "view/edit notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
			edit := cfg.Menu.DefaultKeymaps.Edit
			edit.Hidden = false

			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(edit)),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName, "notes")+" {+1}"),
			)

			return cmdutil.Execute(cmd, args, m, func(d *deps.Deps, bs []*bookmark.Bookmark) error {
				return printer.Notes(d.Console(), bs)
			}, OnlyNotes)
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)
	cmdutil.HideFlag(c, "help")

	c.AddCommand(newEditNotesCmd(cfg))

	return c
}

func newEditNotesCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [query]",
		Short: "edit notes with text editor",
		RunE: func(cmd *cobra.Command, args []string) error {
			kb := menu.NewKeybindBuilder(cfg.Cmd, cfg.DBName)
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithHeader("select record/s"),
				menu.WithBorderLabel(" notes "),
				menu.WithKeybinds(kb.EditNotes(cfg.Menu.DefaultKeymaps.Edit)),
				menu.WithPreview(cfg.PreviewCmd(cfg.DBName, "notes")+" {+1}"),
			)

			return cmdutil.Execute(cmd, args, m, handler.Edit(editor.NotesStrategy{}))
		},
	}

	cmdutil.FlagMenu(c, cfg)
	cmdutil.FlagsFilter(c, cfg)
	cmdutil.HideFlag(c, "help")

	return c
}

func OnlyNotes(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	filtered := make([]*bookmark.Bookmark, 0, len(bs))
	for i := range bs {
		if bs[i].Notes == "" {
			continue
		}

		filtered = append(filtered, bs[i])
	}

	return filtered
}
