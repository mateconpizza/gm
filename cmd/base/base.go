package base

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

const UsageTemplate = `usage: {{if .Runnable}}{{.UseLine}}{{end}}{{if .HasAvailableSubCommands}} [command]{{end}}
{{- if gt (len .Aliases) 0}}

aliases: {{.NameAndAliases}}
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

type BookmarkAction func(*app.Context, []*bookmark.Bookmark) error

// SetupApp inicializa la config, db y app para los subcommands.
func SetupApp(cmd *cobra.Command, args *[]string) (*app.Context, func(), error) {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return nil, nil, err
	}

	terminal.ReadPipedInput(args)

	a := app.New(cmd.Context(),
		app.WithConfig(cfg),
		app.WithDB(r),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	return a, r.Close, nil
}

// resolveBookmarks es el setup adicional compartido entre subcommands que trabajan con records.
func resolveBookmarks(a *app.Context, m *menu.Menu[bookmark.Bookmark], args []string) ([]*bookmark.Bookmark, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("menu: %w", err)
	}

	bs, err := handler.Data(a, m, args)
	if err != nil {
		return nil, err
	}

	if len(bs) == 0 {
		return nil, db.ErrRecordNotFound
	}

	return bs, nil
}

func RunWithBookmarks(cmd *cobra.Command, args []string, m *menu.Menu[bookmark.Bookmark], action BookmarkAction) error {
	a, cleanup, err := SetupApp(cmd, &args)
	if err != nil {
		return err
	}
	defer cleanup()

	bs, err := resolveBookmarks(a, m, args)
	if err != nil {
		return err
	}

	return action(a, bs)
}

// FlagFormat initializes CLI flags for the records command.
func FlagFormat(c *cobra.Command, cfg *config.Config) {
	f := c.Flags()
	f.SortFlags = false

	// Display
	f.StringVarP(&cfg.Flags.Format, "format", "f", "",
		fmt.Sprintf("output format [%s]", strings.Join(printer.ValidFormats, "|")))
}

func FlagsFilter(c *cobra.Command, cfg *config.Config) {
	f := c.Flags()
	f.SortFlags = false
	f.StringSliceVarP(&cfg.Flags.Tags, "tag", "t", nil, "filter by tag(s)")
	f.IntVarP(&cfg.Flags.Head, "head", "H", 0, "show first N bookmarks")
	f.IntVarP(&cfg.Flags.Tail, "tail", "T", 0, "show last N bookmarks")
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
			return
		}
		if f := c.PersistentFlags().Lookup(name); f != nil {
			f.Hidden = true
		}
	}
}
