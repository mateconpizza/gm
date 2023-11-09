/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/constants"
	"gomarks/pkg/errs"

	"github.com/spf13/cobra"
)

var (
	queryEdit    string
	editExamples = []string{"edit <id>\n", "edit <query>"}
)

var editCmd = &cobra.Command{
	Use:     "edit",
	Short:   "edit selected bookmark",
	Example: exampleUsage(editExamples),
	RunE: func(_ *cobra.Command, args []string) error {
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

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		b, err := r.GetRecordByID(constants.DBMainTableName, id)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		b, err = bookmark.Edit(b)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if _, err := r.UpdateRecord(constants.DBMainTableName, b); err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(b.PrettyColorString())

		return nil
	},
}

func init() {
	editCmd.Flags().StringVarP(&queryEdit, "query", "q", "", "query to filter bookmarks")
	rootCmd.AddCommand(editCmd)
}
