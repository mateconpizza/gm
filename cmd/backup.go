// Copyright © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	bkCreate  bool
	bkRemove  bool
	bkRestore bool
	bkStatus  bool
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "backup databases",
	Run: func(cmd *cobra.Command, args []string) {
		if bkCreate {
			fmt.Println("::create::", bkCreate)
		}
		if bkRemove {
			fmt.Println("::remove::", bkRemove)
		}
		if bkRestore {
			fmt.Println("::restore::", bkRestore)
		}
		if bkStatus {
			fmt.Println("::status::", bkStatus)
		}
		fmt.Println("args:", args)
	},
}

func init() {
	backupCmd.Flags().BoolVarP(&bkCreate, "create", "c", false, "create backup")
	backupCmd.Flags().BoolVarP(&bkRemove, "remove", "r", false, "remove backup")
	backupCmd.Flags().BoolVarP(&bkRestore, "restore", "s", false, "restore backup")
	backupCmd.Flags().BoolVarP(&bkStatus, "status", "t", false, "check backup status")
	rootCmd.AddCommand(backupCmd)
}
