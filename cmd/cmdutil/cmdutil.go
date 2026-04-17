package cmdutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
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
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}

	terminal.ReadPipedInput(args)

	d := deps.New(cmd.Context(),
		deps.WithConfig(cfg),
		deps.WithDB(r),
		deps.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	return d, r.Close, nil
}

// resolveBookmarks es el setup adicional compartido entre subcommands que trabajan con records.
func resolveBookmarks(a *deps.Deps, m *menu.Menu[bookmark.Bookmark], args []string) ([]*bookmark.Bookmark, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("menu: %w", err)
	}

	bs, err := handler.Data(a, m, args)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

func Execute(
	c *cobra.Command,
	args []string,
	m *menu.Menu[bookmark.Bookmark],
	action BookmarkAction,
	filters ...Filter,
) error {
	d, cleanup, err := SetupDeps(c, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	bs, err := resolveBookmarks(d, m, args)
	if err != nil {
		return err
	}

	// custom filters
	for _, f := range filters {
		bs = f(bs)
	}

	// filter by head and tail
	f := d.Cfg.Flags
	if f.Head > 0 || f.Tail > 0 {
		bs, err = handler.FilterByHeadAndTail(bs, f.Head, f.Tail)
		if err != nil {
			return fmt.Errorf("failed to filter by head/tail: %w", err)
		}
	}

	// menu selection
	if f.Menu && len(bs) > 0 {
		bs, err = handler.ApplyMenuSelection(d.Console(), m, bs)
		if err != nil {
			return err
		}
	}

	return action(d, bs)
}

func FlagFormat(c *cobra.Command, cfg *config.Config) {
	c.Flags().SortFlags = false
	c.Flags().StringVarP(&cfg.Flags.Format, "format", "f", "",
		fmt.Sprintf("output format [%s]", strings.Join(printer.ValidFormats, "|")))
}

func FlagsFilter(c *cobra.Command, cfg *config.Config) {
	c.Flags().SortFlags = false
	c.Flags().StringSliceVarP(&cfg.Flags.Tags, "tag", "t", nil, "filter by tag(s)")
	c.Flags().IntVarP(&cfg.Flags.Head, "head", "H", 0, "limit to first N bookmarks")
	c.Flags().IntVarP(&cfg.Flags.Tail, "tail", "T", 0, "limit to last N bookmarks")
}

func FlagMenu(c *cobra.Command, cfg *config.Config) {
	c.Flags().BoolVarP(&cfg.Flags.Menu, "menu", "m", false, "select interactively")
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

func FlagDBRequired(c *cobra.Command, cfg *config.Config) {
	c.Flags().StringVar(&cfg.DBName, "db", config.MainDBName, "database name")
	_ = c.MarkFlagRequired("db")
}
