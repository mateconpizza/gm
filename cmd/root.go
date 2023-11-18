/*
Copyright © 2023 haaag <git.haaag@gmail.com>
*/package cmd

import (
	"fmt"
	"log"
	"os"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/display"
	"gomarks/pkg/menu"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

const maxLen = 80

var (
	Menu          *menu.Menu
	formatFlag    string
	headFlag      int
	noConfirmFlag bool
	pickerFlag    string
	tailFlag      int
	verboseFlag   bool
)

var rootCmd = &cobra.Command{
	Use:          config.App.Name,
	Short:        "Gomarks is a bookmark manager for your terminal",
	Long:         "Gomarks is a bookmark manager for your terminal",
	SilenceUsage: true,
	Args:         cobra.MinimumNArgs(0),
	PreRunE:      checkInitDB,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, _ := getDB()

		if len(args) == 0 {
			args = []string{""}
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if Menu != nil {
			var b *bookmark.Bookmark
			b, err = display.SelectBookmark(Menu, bs)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			bs = bookmark.NewSlice(b)
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

	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu|rofi]")
	rootCmd.PersistentFlags().
		StringVarP(&formatFlag, "format", "f", "pretty", "output format [json|pretty]")
	rootCmd.PersistentFlags().
		StringVarP(&pickerFlag, "pick", "p", "", "pick oneline data [id|url|title|tags]")

	rootCmd.PersistentFlags().
		IntVarP(&headFlag, "head", "H", 0, "the <int> first part of bookmarks")
	rootCmd.PersistentFlags().
		IntVarP(&tailFlag, "tail", "T", 0, "the <int> last part of bookmarks")

	rootCmd.SilenceErrors = true
}

func initConfig() {
	util.SetLogLevel(&verboseFlag)

	var err error
	Menu, err = handleMenu()
	if err != nil {
		log.Fatal(err)
	}
}
