package cmdutil

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mateconpizza/gm/internal/application"
)

var GlobalFlags = []string{"fields", "head", "menu", "output", "sort", "tag", "tail"}

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

func FlagOutput(c *cobra.Command, app *application.App, supportedOutput []string) {
	c.Flags().StringVarP(&app.Flags.Output, "output", "o", "", "output format: "+strings.Join(supportedOutput, ", "))
}

func FlagFields(c *cobra.Command, app *application.App, fields string) {
	c.Flags().StringVarP(&app.Flags.Field, "fields", "f", "", "select fields: "+fields)
}

func FlagDBRequired(c *cobra.Command, app *application.App) {
	c.Flags().StringVar(&app.DBName, "db", app.DBName, "database name")
	_ = c.MarkFlagRequired("db")
}

func FlagsFilter(c *cobra.Command, app *application.App) {
	c.Flags().StringSliceVarP(&app.Flags.Tags, "tag", "t", nil, "filter by tag(s)")
	c.Flags().IntVarP(&app.Flags.Head, "head", "H", 0, "limit to first N bookmarks")
	c.Flags().IntVarP(&app.Flags.Tail, "tail", "T", 0, "limit to last N bookmarks")
}

func FlagMenu(c *cobra.Command, app *application.App) {
	c.Flags().BoolVarP(&app.Flags.Menu, "menu", "m", false, "select interactively")
}

func FlagSort(c *cobra.Command, app *application.App, sortSupported []string) {
	c.Flags().StringVarP(&app.Flags.Sort, "sort", "s", "", "sort by: "+strings.Join(sortSupported, ", "))
}

func HasFlags(c *cobra.Command) bool {
	hasVisible := false
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			hasVisible = true
		}
	})
	return hasVisible
}

func HideInheritedFlags(c *cobra.Command) {
	c.SetHelpFunc(func(c *cobra.Command, args []string) {
		c.InheritedFlags().VisitAll(func(f *pflag.Flag) {
			f.Hidden = true
		})

		c.Parent().HelpFunc()(c, args)
	})
}

func HideFlag(c *cobra.Command, names ...string) {
	for _, name := range names {
		// search local flags
		if f := c.Flags().Lookup(name); f != nil {
			f.Hidden = true
			continue
		}
		// search in flags inherited from the parent
		if f := c.InheritedFlags().Lookup(name); f != nil {
			f.Hidden = true
			continue
		}
		// not found?: register as local and hide
		c.Flags().Bool(name, false, "")
		_ = c.Flags().MarkHidden(name)
	}
}

// DisableFlagSorting recursively disables flag sorting on a command
// and all its subcommands.
func DisableFlagSorting(c *cobra.Command) *cobra.Command {
	c.Flags().SortFlags = false
	c.InheritedFlags().SortFlags = false
	c.PersistentFlags().SortFlags = false
	for _, sub := range c.Commands() {
		DisableFlagSorting(sub)
	}
	return c
}
