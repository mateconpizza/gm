package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

type (
	// BookmarkAction defines a task to be performed on a set of bookmarks.
	BookmarkAction func(*deps.Deps, []*bookmark.Bookmark) error

	// Filter is a predicate used to narrow down a slice of bookmarks
	// before they are passed to an action or presented in a menu.
	Filter func([]*bookmark.Bookmark) []*bookmark.Bookmark
)

const UsageTemplate = `usage: {{if .Runnable}}{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}} [command]{{end}}
{{- if gt (len .Aliases) 0}}

aliases: {{.NameAndAliases}}
{{- end}}
{{- if .HasExample}}

examples:
{{.Example}}
{{- end}}
{{- if gt (len .Commands) 0}}

commands:
{{- range .Commands}}
  {{- if or .IsAvailableCommand (eq .Name "help")}}
  {{rpad .Name .NamePadding}} {{.Short}}
  {{- end}}
{{- end}}
{{- end}}
{{- if hasFlags .}}

flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
{{- if .HasAvailableInheritedFlags}}

global:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}
{{- end}}
`

// SetupDeps inicializa la config, db y app para los subcommands.
func SetupDeps(cmd *cobra.Command, args *[]string) (*deps.Deps, func(), error) {
	app, err := application.FromContext(cmd.Context())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(app.Path.Database)
	if err != nil {
		return nil, nil, err
	}

	terminal.ReadPipedInput(args)

	d := deps.New(cmd.Context(),
		deps.WithApplication(app),
		deps.WithRepo(r),
		deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	return d, r.Close, nil
}

func Execute(
	cmd *cobra.Command,
	args []string,
	m *menu.Menu[bookmark.Bookmark],
	action BookmarkAction,
	filters ...Filter,
) error {
	d, cleanup, err := SetupDeps(cmd, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	bs, err := handler.Data(d, args)
	if err != nil {
		return err
	}

	// custom filters
	for _, filter := range filters {
		bs = filter(bs)
	}

	// filter by head and tail
	f := d.App.Flags
	if f.Head > 0 || f.Tail > 0 {
		bs, err = handler.FilterByHeadAndTail(bs, f.Head, f.Tail)
		if err != nil {
			return fmt.Errorf("failed to filter by head/tail: %w", err)
		}
	}

	// menu selection
	if f.Menu && len(bs) > 0 {
		bs, err = handler.MenuSelection(m, bs)
		if err != nil {
			return err
		}
	}

	return action(d, bs)
}

func FlagOutput(c *cobra.Command, app *application.App, supportedOutput []string) {
	c.Flags().StringVarP(&app.Flags.Output, "output", "o", "", "output format: "+strings.Join(supportedOutput, ", "))

	_ = c.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return supportedOutput, cobra.ShellCompDirectiveNoFileComp
		})
}

func FlagFields(c *cobra.Command, app *application.App, fields string) {
	c.Flags().StringVarP(&app.Flags.Field, "fields", "f", "", "select fields: "+fields)
}

func HasFlags(cmd *cobra.Command) bool {
	hasVisible := false
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			hasVisible = true
		}
	})
	return hasVisible
}

func HideFlag(c *cobra.Command, names ...string) {
	for _, name := range names {
		if f := c.Flags().Lookup(name); f != nil {
			f.Hidden = true
			continue
		}
		if f := c.PersistentFlags().Lookup(name); f != nil {
			f.Hidden = true
		}
	}
}

func FlagDBRequired(c *cobra.Command, app *application.App) {
	c.Flags().StringVar(&app.DBName, "db", application.MainDBName, "database name")
	_ = c.MarkFlagRequired("db")
}
