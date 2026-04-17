package tag

import (
	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/ui/printer"
)

// NewCmd manages bookmark tags (list, JSON export, etc.).
func NewCmd(cfg *config.Config) *cobra.Command {
	c := &cobra.Command{
		Use:     "tag",
		Aliases: []string{"t", "tags"},
		Short:   "tags ops (wip)",
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

	c.Flags().BoolVarP(&cfg.Flags.JSON, "json", "j", false,
		"output tags+count in JSON format")
	c.Flags().BoolVarP(&cfg.Flags.List, "list", "l", false,
		"list all tags")

	return c
}
