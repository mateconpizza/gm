package tags

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

// NewCmd manages bookmark tags (list, JSON export, etc.).
func NewCmd(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tags",
		Aliases: []string{"t"},
		Short:   "Tags management",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case cfg.Flags.JSON:
				return printer.TagsJSON(cmd.Context(), cfg.DBPath)
			case cfg.Flags.List:
				return printer.TagsList(cmd.Context(), cfg.DBPath)
			}

			return cmd.Usage()
		},
	}

	cmd.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false,
		"output tags+count in JSON format")
	cmd.Flags().BoolVarP(&cfg.Flags.List, "list", "l", false,
		"list all tags")

	return cmd
}
