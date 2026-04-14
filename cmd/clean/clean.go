package clean

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:   "clean [query]",
		Short: "strip URL params",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, cancel, err := cmdutil.SetupApp(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			bs, err := a.DB.All(a.Context())
			if err != nil {
				return err
			}

			bs = handler.ParamsFilter(bs)
			if len(bs) == 0 {
				return fmt.Errorf("items with %w", handler.ErrURLParamsNotFound)
			}

			if a.Cfg.Flags.Menu {
				bs, err = paramsMenu(a, bs)
				if err != nil {
					return err
				}
			}

			return handler.ParamsURL(a, bs)
		},
	}

	cmdutil.FlagMenu(c, cfg)
	c.Flags().Bool("help", false, "help message")
	_ = c.Flags().MarkHidden("help")

	return c
}

// paramsMenu presents the given bookmarks in an interactive menu with
// highlighted URL parameters.
func paramsMenu(a *app.Context, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	m := handler.MenuSimple[*bookmark.Bookmark](
		cfg,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" parameters highlighted "),
		menu.WithPreview(cfg.PreviewCmd(cfg.DBName)+" {1}"),
	)

	m.SetFormatter(func(b **bookmark.Bookmark) string {
		bm := *b
		bm.URL = handler.ParamHighlight(bm.URL, ansi.BrightRed, ansi.Italic)
		return txt.OnelineURL(a.Console(), bm)
	})

	bs, err = m.Select(bs)
	if err != nil {
		return nil, err
	}

	cleaned := make([]*bookmark.Bookmark, 0, len(bs))
	for _, bm := range bs {
		bm.URL = ansi.Remover(bm.URL)
		cleaned = append(cleaned, bm)
	}

	return cleaned, nil
}
