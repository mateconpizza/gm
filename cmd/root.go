/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/

package cmd

import (
	"fmt"
	"os"

	"gomarks/pkg/config"
	"gomarks/pkg/display"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

// TODO:
// [ ] - make `maxLen` global and flag

const maxLen = 80

var (
	formatFlag    string
	headFlag      int
	infoFlag      bool
	noConfirmFlag bool
	pickerFlag    string
	tailFlag      int
	verboseFlag   bool
)

var rootCmd = &cobra.Command{
	Use:          config.App.Name,
	Short:        config.App.Desc,
	Long:         config.App.Desc,
	SilenceUsage: true,
	Args:         cobra.MinimumNArgs(0),
	PreRunE:      checkInitDB,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, _ := getDB()

		if len(args) == 0 {
			args = []string{""}
		}

		if err := handleInfoFlag(cmd, r); err != nil {
			return fmt.Errorf("%w", err)
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if bs, err = display.Select(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleHeadAndTail(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handlePicker(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if bs.Len() == 1 {
			util.CopyToClipboard((*bs)[0].URL)
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Printf("%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}
}

func init() {
	var menuFlag string

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVar(&noConfirmFlag, "no-confirm", false, "no confirm mode")
	rootCmd.PersistentFlags().BoolVarP(&infoFlag, "info", "i", false, "show app info")

	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu|rofi]")
	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "pretty", "output format [json|pretty]")
	rootCmd.PersistentFlags().StringVarP(&pickerFlag, "pick", "p", "", "pick oneline data [id|url|title|tags]")

	rootCmd.PersistentFlags().IntVar(&headFlag, "head", 0, "the <int> first part of bookmarks")
	rootCmd.PersistentFlags().IntVar(&tailFlag, "tail", 0, "the <int> last part of bookmarks")

	rootCmd.SilenceErrors = true
}

func initConfig() {
	util.SetLogLevel(&verboseFlag)
}
