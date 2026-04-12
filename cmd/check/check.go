// Package check provides bookmark health verification and maintenance.
package check

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/base"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c"},
		Short:   "check URLs",
		Annotations: map[string]string{
			"group": "management",
		},
		RunE: checkerFunc,
	}

	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	base.FlagsFilter(c, cfg)

	c.AddCommand(newSnapCmd(cfg))
	c.AddCommand(newStatusCmd(cfg))
	c.AddCommand(newUpdateCmd(cfg))

	return c
}

func checkerFunc(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	a := app.New(cmd.Context(),
		app.WithDB(r),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	m := handler.MenuSimple[bookmark.Bookmark](cfg,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s for checking status"))
	bs, err := handler.Data(a, m, args)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	f := a.Cfg.Flags
	switch {
	case f.Status: // FIX: remove
		return handler.CheckStatus(a, bs)
	case f.Update:
		return handler.Update(a, bs)
	}

	return cmd.Help()
}

func newStatusCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "status [id|query]",
		Short: "check URLs HTTP status",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithBorderLabel(" "+config.AppName+" "),
				menu.WithHeader("select record/s"),
				menu.WithHeaderBorder(menu.BorderRounded),
				menu.WithPreviewBorder(menu.BorderRounded),
				menu.WithHeaderFirst(),
			)

			return base.RunWithBookmarks(cmd, args, m, handler.CheckStatus)
		},
	}

	return c
}

func newUpdateCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "update [id|query]",
		Short: "update metadata (title|desc|tags)",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := handler.MenuSimple[bookmark.Bookmark](cfg,
				menu.WithMultiSelection(),
				menu.WithBorderLabel(" "+config.AppName+" "),
				menu.WithHeader("select record/s"),
				menu.WithHeaderBorder(menu.BorderRounded),
				menu.WithPreviewBorder(menu.BorderRounded),
				menu.WithHeaderFirst(),
			)

			return base.RunWithBookmarks(cmd, args, m, handler.Update)
		},
	}

	return c
}
