package tag

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

// NewCmd manages bookmark tags (list, JSON export, etc.).
func NewCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "tag",
		Aliases: []string{"t", "tags"},
		Short:   "tags operations (wip)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			switch {
			case app.Flags.JSON:
				return printer.TagsJSON(ctx, os.Stdout, app.Path.DB())
			case app.Flags.List:
				return printer.TagsList(ctx, os.Stdout, app.Path.DB())
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
