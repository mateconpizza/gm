package cmdutil

import (
	"path/filepath"
	"strings"

	files "github.com/mateconpizza/gofiles"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/pkg/db"
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

func FlagOutput(c *cobra.Command, app *application.App, def string, supportedOutput []string) {
	c.Flags().StringVarP(&app.Flags.Output, "output", "o", def,
		"output format: "+strings.Join(supportedOutput, ", "))

	FlagCompletion(c, "output", func(cmd *cobra.Command) ([]string, error) {
		return supportedOutput, nil
	})
}

func FlagFields(c *cobra.Command, app *application.App, fields string) {
	c.Flags().StringVarP(&app.Flags.Field, "fields", "f", "", "select fields: "+fields)

	FlagCompletion(c, "fields", func(cmd *cobra.Command) ([]string, error) {
		return strings.Split(fields, ","), nil
	})
}

func FlagDBRequired(c *cobra.Command, app *application.App) {
	c.Flags().StringVar(&app.DBName, "db", app.DBName, "database name")
	_ = c.MarkFlagRequired("db")
}

func FlagDatabase(c *cobra.Command, app *application.App) {
	g := c.PersistentFlags()
	g.StringVar(&app.DBName, "db", app.DBName, "database name")

	items, _ := files.FindByExtension(app.Path.Home(), ".db")
	dbs := make([]string, 0, len(items))
	for _, f := range items {
		base := filepath.Base(f)
		dbs = append(dbs, strings.TrimSuffix(base, ".db"))
	}

	FlagCompletion(c, "db", func(cmd *cobra.Command) ([]string, error) {
		return dbs, nil
	})
}

func FlagsFilter(c *cobra.Command, app *application.App) {
	c.Flags().StringSliceVarP(&app.Flags.Tags, "tag", "t", nil, "filter by tag(s)")
	c.Flags().IntVarP(&app.Flags.Head, "head", "H", 0, "limit to first N bookmarks")
	c.Flags().IntVarP(&app.Flags.Tail, "tail", "T", 0, "limit to last N bookmarks")

	FlagCompletion(c, "tag", tagFetcher(app))
}

func FlagMenu(c *cobra.Command, app *application.App) {
	c.Flags().BoolVarP(&app.Flags.Menu, "menu", "m", false, "select interactively")
}

func FlagSort(c *cobra.Command, app *application.App, sortSupported []string) {
	c.Flags().StringVarP(&app.Flags.Sort, "sort", "s", "", "sort by: "+strings.Join(sortSupported, ", "))

	FlagCompletion(c, "sort", func(cmd *cobra.Command) ([]string, error) {
		return sortSupported, nil
	})
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

type FlagItemsFetcher func(cmd *cobra.Command) ([]string, error)

func FlagCompletion(c *cobra.Command, flag string, fetch FlagItemsFetcher) {
	_ = c.RegisterFlagCompletionFunc(
		flag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			items, err := fetch(cmd)
			if err != nil {
				return nil, cobra.ShellCompDirectiveError
			}
			return items, cobra.ShellCompDirectiveNoFileComp
		},
	)
}

func tagFetcher(app *application.App) FlagItemsFetcher {
	return func(cmd *cobra.Command) ([]string, error) {
		dbName := app.DBName

		if cmd.Flags().Changed("db") {
			if v, err := cmd.Flags().GetString("db"); err == nil {
				dbName = v
			}
		}

		dbPath := filepath.Join(app.Path.Data, files.EnsureExt(dbName, ".db"))
		if !files.Exists(dbPath) {
			return nil, nil
		}

		r, err := db.New(cmd.Context(), dbPath)
		if err != nil {
			return nil, err
		}
		defer r.Close()

		tags, err := db.TagsList(cmd.Context(), r)
		if err != nil {
			return nil, err
		}

		return tags, nil
	}
}
