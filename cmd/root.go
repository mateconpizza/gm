// Copyrighs © 2023 haaag <git.haaag@gmail.com>
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"

	"gomarks/pkg/bookmark"
	"gomarks/pkg/config"
	"gomarks/pkg/format"
	"gomarks/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	addFlag     bool
	colorFlag   string
	copyFlag    bool
	openFlag    bool
	editionFlag bool
	forceFlag   bool
	formatFlag  string
	headFlag    int
	infoFlag    bool
	isPiped     bool
	listFlag    bool
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
	RunE: func(_ *cobra.Command, args []string) error {
		r, err := bookmark.NewRepository()
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		defer func() {
			if err := r.DB.Close(); err != nil {
				log.Printf("closing database: %v", err)
			}
		}()

		parseArgsAndExit(r)

		if len(args) == 0 && !addFlag {
			args = append(args, "")
		}

		terminal.ReadInputFromPipe(&args)

		if addFlag {
			return handleAdd(r, args)
		}

		bs, err := handleFetchRecords(r, args)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		if len(*bs) == 0 {
			return bookmark.ErrBookmarkNotFound
		}

		filteredBs, err := handleHeadAndTail(bs)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		handleBookmarksAndExit(r, &filteredBs)

		if err := handlePicker(&filteredBs); err != nil {
			return fmt.Errorf("%w", err)
		}

		if err := handleFormat(&filteredBs); err != nil {
			return fmt.Errorf("%w", err)
		}

		bookmarkSelected := filteredBs[0]
		if copyFlag {
			if err := bookmarkSelected.Copy(); err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		if openFlag {
			if err := bookmarkSelected.Open(); err != nil {
				return fmt.Errorf("%w", err)
			}
		}

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()

	if errors.Is(err, bookmark.ErrDBNotFound) {
		init := format.Text("init").Yellow().Bold()
		err = fmt.Errorf("%w: use %s to initialize a new database", err, init)
	}

	if err != nil {
		logErrAndExit(err)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().BoolVarP(&verboseFlag, "verbose", "v", false, "verbose mode")
	rootCmd.Flags().BoolVar(&infoFlag, "info", false, "show app info")
	rootCmd.Flags().BoolVar(&versionFlag, "version", false, "print version info")

	// Actions
	rootCmd.Flags().BoolVar(&copyFlag, "copy", true, "copy bookmark to clipboard")
	rootCmd.Flags().BoolVar(&openFlag, "open", false, "open bookmark in default browser")
	rootCmd.PersistentFlags().BoolVar(&forceFlag, "force", false, "force action")

	// Experimental
	rootCmd.Flags().BoolVarP(&listFlag, "list", "l", false, "list bookmarks")
	rootCmd.Flags().BoolVarP(&editionFlag, "edition", "e", false, "edition mode")
	rootCmd.Flags().BoolVarP(&statusFlag, "status", "s", false, "check bookmarks status")
	rootCmd.Flags().StringVar(&colorFlag, "color", "always", "print with pretty colors [always|never]")

	// More experimental
	rootCmd.Flags().BoolVarP(&removeFlag, "remove", "r", false, "remove a bookmarks by query or id")
	rootCmd.Flags().BoolVarP(&addFlag, "add", "a", false, "add a new bookmark")

	rootCmd.Flags().StringVarP(&formatFlag, "format", "f", "pretty", "output format [json|pretty]")
	rootCmd.Flags().StringVarP(&pickerFlag, "pick", "p", "", "pick oneline data [id|url|title|tags]")

	// Modifiers
	rootCmd.Flags().IntVar(&headFlag, "head", 0, "the <int> first part of bookmarks")
	rootCmd.Flags().IntVar(&tailFlag, "tail", 0, "the <int> last part of bookmarks")

	rootCmd.SilenceErrors = true
}

func initConfig() {
	setLoggingLevel(&verboseFlag)

	if err := terminal.LoadDefaults(colorFlag); err != nil {
		fmt.Printf("%s: %s\n", config.App.Name, err)
		os.Exit(1)
	}

	isPiped = terminal.IsPiped()
}
