package out

import (
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/cli"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookio"
	"github.com/mateconpizza/gm/pkg/bookmark"
)

func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "export [id|query]",
		Short:   "export bookmarks",
		Aliases: []string{"ex"},
		RunE:    cli.HookHelp,
	}

	cmds := []func(*application.App) *cobra.Command{newHTMLCmd, newJSONCmd, newCSVCmd}
	for i := range cmds {
		cmd := cmds[i](app)
		cmdutil.HideFlag(cmd, "help")
		c.AddCommand(cmd)
	}

	return c
}

func newHTMLCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "html [id|query]",
		Short: "export to HTML Netscape",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to HTML ")
			return cmdutil.Execute(cmd, args, m, func(_ *deps.Deps, bs []*bookmark.Bookmark) error {
				return bookio.ExportToNetscapeHTML(bs, os.Stdout)
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func newJSONCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "json [id|query]",
		Short: "export to JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to JSON ")
			return cmdutil.Execute(cmd, args, m, func(_ *deps.Deps, bs []*bookmark.Bookmark) error {
				return printer.RecordsJSON(bs)
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)

	return c
}

func newCSVCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:   "csv [id|query]",
		Short: "export to CSV",
		RunE: func(cmd *cobra.Command, args []string) error {
			m := setupMenu(app, " export to CSV ")
			return cmdutil.Execute(cmd, args, m, func(_ *deps.Deps, bs []*bookmark.Bookmark) error {
				return bookio.ExportToCSV(bs, os.Stdout, parseCSVFields(app.Flags.Field))
			})
		},
	}

	cmdutil.FlagMenu(c, app)
	cmdutil.FlagsFilter(c, app)
	cmdutil.FlagFields(c, app, "all,"+wrapFields(bookmark.Fields(), ",", 50))

	return c
}

func wrapFields(fields []string, sep string, maxLen int) string {
	var sb strings.Builder
	line := ""

	for i, f := range fields {
		part := f
		if i < len(fields)-1 {
			part += sep
		}

		if len(line)+len(part) > maxLen && line != "" {
			sb.WriteString(line + "\n")
			line = part
		} else {
			line += part
		}
	}

	sb.WriteString(line)
	return sb.String()
}

// parseCSVFields normalises the --fields flag value into a deduplicated,
// lowercase slice of field names ready for ExportToCSV.
//
//   - empty string  → CSVDefaultHeader
//   - "all"         → bookmark.Fields()
//   - "id,URL, url" → ["id", "url"]  (trimmed, lowercased, deduplicated)
func parseCSVFields(f string) []string {
	f = strings.TrimSpace(f)
	if f == "" {
		return bookio.CSVDefaultHeader
	}

	f = strings.Trim(f, ",")
	parts := strings.Split(f, ",")

	// Normalise each part.
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		// A single "all" token anywhere in the list wins immediately.
		if p == "all" {
			return bookmark.Fields()
		}
		if _, dup := seen[p]; !dup {
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}

	return out
}

func setupMenu(app *application.App, label string) *menu.Menu[bookmark.Bookmark] {
	return handler.MenuSimple[bookmark.Bookmark](app,
		menu.WithMultiSelection(),
		menu.WithHeader("select record/s"),
		menu.WithHeaderLabel(label),
		menu.WithPreview(app.PreviewCmd(app.DBName, "{1}")),
	)
}
