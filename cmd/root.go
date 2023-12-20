// Copyrighs © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"os"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/display"
	"gomarks/pkg/format"
	"gomarks/pkg/util"

	"github.com/spf13/cobra"
)

var (
	allFlag     bool
	editionFlag bool
	formatFlag  string
	headFlag    int
	infoFlag    bool
	pickerFlag  string
	statusFlag  bool
	tailFlag    int
	verboseFlag bool
	versionFlag bool
)

var rootCmd = &cobra.Command{
	Use:          config.App.Cmd,
	Short:        config.App.Data.Desc,
	Long:         config.App.Data.Desc,
	SilenceUsage: true,
	Args:         cobra.MinimumNArgs(0),
	PreRunE:      checkInitDB,
	RunE: func(cmd *cobra.Command, args []string) error {
		r, _ := getDB()

		parseArgsAndExit(cmd, r)

		args = handleAllFlag(cmd, args)

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

		parseBookmarksAndExit(cmd, bs)

		if err := handlePicker(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if len(*bs) == 1 {
			util.CopyToClipboard((*bs)[0].URL)
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()

	if errors.Is(err, bookmark.ErrDBNotFound) {
		err = fmt.Errorf("%w: use %s to initialize a new database", err, format.Text("init").Yellow().Bold())
	}

	if err != nil {
		logErrAndExit(err)
	}
}

func init() {
	var menuFlag string

	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose mode")
	rootCmd.PersistentFlags().BoolVarP(&infoFlag, "info", "i", false, "show app info")
	rootCmd.Flags().BoolVar(&versionFlag, "version", false, "print version info")

	// Experimental
	rootCmd.Flags().BoolVarP(&allFlag, "all", "a", false, "all bookmarks")
	rootCmd.Flags().BoolVarP(&statusFlag, "status", "s", false, "check bookmarks status")

	rootCmd.PersistentFlags().StringVarP(&menuFlag, "menu", "m", "", "menu mode [dmenu|rofi]")
	rootCmd.PersistentFlags().StringVarP(&formatFlag, "format", "f", "pretty", "output format [json|pretty]")
	rootCmd.PersistentFlags().StringVarP(&pickerFlag, "pick", "p", "", "pick oneline data [id|url|title|tags]")

	rootCmd.PersistentFlags().IntVar(&headFlag, "head", 0, "the <int> first part of bookmarks")
	rootCmd.PersistentFlags().IntVar(&tailFlag, "tail", 0, "the <int> last part of bookmarks")

	rootCmd.SilenceErrors = true
}

func initConfig() {
	util.SetLogLevel(&verboseFlag)

	if err := handleTermOptions(); err != nil {
		fmt.Printf("%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}
}
