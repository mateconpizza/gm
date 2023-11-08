/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"strconv"

	"gomarks/pkg/errs"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var ID int

var deleteCmd = &cobra.Command{
	Use:          "delete",
	Short:        "delete a bookmark by id",
	Long:         "delete a bookmark by id",
	Example:      "  gomarks delete <bookmark_id>",
	SilenceUsage: true,
	Args:         cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		util.CmdTitle("deleting a bookmark")

		id, err := strconv.Atoi(args[0])
		if err != nil || id <= 0 {
			return fmt.Errorf("%w", errs.ErrNoIDProvided)
		}

		r, err := getDB()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		b, err := r.GetRecordByID(id, "bookmarks")
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		fmt.Println(b.PrettyColorString())
		confirm := util.ConfirmChanges("Delete bookmark?")
		if !confirm {
			return fmt.Errorf("%w", errs.ErrActionAborted)
		}

		fmt.Println("delete bookmark:", b.URL)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
