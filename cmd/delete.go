/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/
package cmd

import (
	"fmt"
	"strconv"

	"gomarks/pkg/errs"

	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete selected bookmark",
	Long:  "delete selected bookmark",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("CMD::::%+v\n", cmd)
		var err error
		if idFlag == 0 && len(args) == 0 {
			fmt.Println(errs.ErrNoIDProvided)
			return
		}

		idFlag, err = strconv.Atoi(args[0])
		if err != nil {
			fmt.Println("err converting idFlag:", err)
		}

		fmt.Println("IDFLAG::::", idFlag)

		if Menu != nil {
			fmt.Println("Menu selected::::", Menu)
		}
	},
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}
