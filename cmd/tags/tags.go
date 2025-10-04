package tags

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

// NewCmd manages bookmark tags (list, JSON export, etc.).
func NewCmd() *cobra.Command {
	app := config.New()

	cmd := &cobra.Command{
		Use:     "tags",
		Aliases: []string{"t"},
		Short:   "Tags management",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case app.Flags.JSON:
				return printer.TagsJSON(app.DBPath)
			case app.Flags.List:
				return printer.TagsList(app.DBPath)
			}

			return cmd.Usage()
		},
	}

	cmd.Flags().BoolVarP(&app.Flags.JSON, "json", "j", false,
		"output tags+count in JSON format")
	cmd.Flags().BoolVarP(&app.Flags.List, "list", "l", false,
		"list all tags")

	return cmd
}
