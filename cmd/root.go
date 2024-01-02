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
	colorFlag   string
	editionFlag bool
	forceFlag   bool
	formatFlag  string
	headFlag    int
	infoFlag    bool
	pickerFlag  string
	removeFlag  bool
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

		parseArgsAndExit(r)

		if allFlag {
			args = append(args, "")
		}

		bs, err := handleGetRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if bs, err = display.Select(cmd, bs); err != nil {
			return fmt.Errorf("%w", err)
		}

		filteredBs, err := handleHeadAndTail(bs)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		parseBookmarksAndExit(r, &filteredBs)

		if err := handlePicker(&filteredBs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(&filteredBs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if len(filteredBs) == 1 {
			util.CopyToClipboard((filteredBs)[0].URL)
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
	rootCmd.Flags().BoolVarP(&editionFlag, "edition", "e", false, "edition mode")
	rootCmd.Flags().BoolVarP(&statusFlag, "status", "s", false, "check bookmarks status")
	rootCmd.Flags().StringVar(&colorFlag, "color", "always", "print with pretty colors [always|never]")
	// More experimental
	rootCmd.Flags().BoolVarP(&removeFlag, "remove", "r", false, "remove a bookmarks by query or id")
	rootCmd.Flags().BoolVar(&forceFlag, "force", false, "force action")

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
