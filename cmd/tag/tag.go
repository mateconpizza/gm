package tag

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

// NewCmd manages bookmark tags (list, JSON export, etc.).
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tag",
		Aliases: []string{"t", "tags"},
		Short:   "tags ops (wip)",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case app.Flags.JSON:
				return printer.TagsJSON(cmd.Context(), app.Path.Database)
			case app.Flags.List:
				return printer.TagsList(cmd.Context(), app.Path.Database)
			}

			return cmd.Usage()
		},
	}

	c.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false,
		"output tags+count in JSON format")
	c.Flags().BoolVarP(&app.Flags.List, "list", "l", false,
		"list all tags")

	return c
}
