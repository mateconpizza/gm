/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"errors"
	"fmt"

	"gomarks/pkg/actions"
	"gomarks/pkg/color"
	"gomarks/pkg/constants"
	"gomarks/pkg/database"
	"gomarks/pkg/errs"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize a new bookmarks database and table",
	Long:  "initialize a new bookmarks database and table",
	RunE: func(_ *cobra.Command, _ []string) error {
		r, err := database.GetDB()

		if errors.Is(err, errs.ErrDBNotFound) {
			home := util.SetupHomeProject()
			fmt.Printf("%sdatabase%s created at:%s", color.Bold, home, color.Reset)
			r.InitDB()
		}

		bs, err := r.GetRecordsAll(constants.DBMainTableName)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		err = actions.HandleFormat("pretty", bs)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
