package clean

import (
	"net/url"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/picker"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/formatter"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "clean [query|URL]",
		Short: "strip URL params",
		RunE: func(cmd *cobra.Command, args []string) error {
			if terminal.IsPiped() {
				terminal.ReadPipedInput(&args)
			}

			if len(args) != 0 && handler.ValidURL(args[0]) || app.Flags.Vacuum {
				return newCleanURLUser(app).RunE(cmd, args)
			}

			return cmdutil.Execute(
				cmd,
				args,
				setupMenu(app),
				handler.ParamsURL,
				WithURLParametersOnly,
			)
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	c.PersistentFlags().BoolVarP(&app.Flags.Vacuum, "all", "a", false, "remove all parameters")

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

// newCleanURLUser takes URL from input and strip useless parameters.
func newCleanURLUser(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "text [url]",
		Short: "strip URL params from input",
		RunE: func(cmd *cobra.Command, args []string) error {
			return handler.ParamsUserInput(cmd.Context(), app, ui.DefaultConsole, args)
		},
	}
	return c
}

func setupMenu(app *application.App) *menu.Menu[bookmark.Bookmark] {
	m := picker.New[bookmark.Bookmark](
		app,
		menu.WithMultiSelection(),
		menu.WithArgs("--cycle"),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(" parameters highlighted "),
		menu.WithPreview(menu.PreviewCmd(app.Command(), app.DBBaseName(), "{1}")),
	)

	m.SetFormatter(func(b *bookmark.Bookmark) string {
		bm := *b
		bm.URL = handler.ParamHighlight(bm.URL, ansi.BrightRed, ansi.Italic)
		return formatter.OnelineURLFunc(ui.NewConsole(), &bm)
	})

	return m
}
