package clean

import (
	"net/url"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/txt"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "clean [query]",
		Short: "strip URL params",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			m := handler.MenuSimple[bookmark.Bookmark](
				app,
				menu.WithMultiSelection(),
				menu.WithArgs("--cycle"),
				menu.WithHeader("select record/s"),
				menu.WithHeaderLabel(" parameters highlighted "),
				menu.WithNth("3.."),
				menu.WithPreview(app.PreviewCmd(app.DBName)+" {1}"),
			)

			m.SetFormatter(func(b *bookmark.Bookmark) string {
				bm := *b
				bm.URL = handler.ParamHighlight(bm.URL, ansi.BrightRed, ansi.Italic)
				return txt.OnelineURL(d.Console(), &bm)
			})

			return cmdutil.Execute(cmd, args, m, handler.ParamsURL, WithURLParametersOnly)
		},
	}

	cmdutil.FlagMenu(c, app)
	c.Flags().Bool("help", false, "help message")
	cmdutil.HideFlag(c, "help")

	return c
}

// WithURLParametersOnly returns bookmarks that contain URL query parameters.
func WithURLParametersOnly(bs []*bookmark.Bookmark) []*bookmark.Bookmark {
	cleanup := make([]*bookmark.Bookmark, 0, len(bs))

	for i := range bs {
		u, err := url.Parse(bs[i].URL)
		if err != nil {
			continue
		}

		if len(u.Query()) == 0 {
			continue
		}

		cleanup = append(cleanup, bs[i])
	}

	return cleanup
}
