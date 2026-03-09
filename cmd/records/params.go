package records

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

func newParamsCmd(cfg *config.Config) *cobra.Command {
	paramsCmd := &cobra.Command{
		Use:     "params [query]",
		Aliases: []string{"p", "par"},
		RunE:    cleanParamsFunc,
	}

	f := paramsCmd.Flags()
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "interactive menu mode using fzf")

	return paramsCmd
}

func cleanParamsFunc(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	a := app.New(ctx,
		app.WithConfig(cfg), app.WithDB(r), app.WithConsole(ui.NewDefaultConsole(ctx, func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	bs, err := r.All(a.Context())
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
}

// paramsMenu presents the given bookmarks in an interactive menu with
// highlighted URL parameters.
func paramsMenu(a *app.Context, bs []*bookmark.Bookmark) ([]*bookmark.Bookmark, error) {
	cfg, err := a.Config()
	if err != nil {
		return nil, err
	}

	m := handler.MenuSimple[*bookmark.Bookmark](cfg,
		menu.WithMultiSelection(), menu.WithHeader("Parameters highlighted"))

	m.SetPreprocessor(func(b **bookmark.Bookmark) string {
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
