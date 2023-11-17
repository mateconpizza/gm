/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"fmt"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/color"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var editExamples = []string{"edit <id>\n", "edit <query>"}

var editCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit selected bookmark",
	Example: exampleUsage(editExamples),
	RunE: func(cmd *cobra.Command, args []string) error {
		var id int
		var err error

		if len(args) > 0 {
			id, err = strconv.Atoi(args[0])
		}

		if err != nil {
			if errors.Is(err, strconv.ErrSyntax) {
				return fmt.Errorf("%w", errs.ErrNoIDProvided)
			}
			return fmt.Errorf("%w", err)
		}

		if id == 0 {
			return fmt.Errorf("%w", errs.ErrNoIDProvided)
		}

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		id := (*bs)[0].ID

		b, err := r.GetRecordByID(config.DB.MainTable, id)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		b, err = bookmark.Edit(b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if _, err := r.UpdateRecord(config.DB.MainTable, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(cmd, &bookmark.Slice{*b}); err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(editCmd)
}
