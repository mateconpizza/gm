package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/haaag/gm/internal/bookmark"
	"github.com/haaag/gm/internal/config"
	"github.com/haaag/gm/internal/format"
	"github.com/haaag/gm/internal/format/color"
	"github.com/haaag/gm/internal/format/frame"
	"github.com/haaag/gm/internal/handler"
	"github.com/haaag/gm/internal/menu"
	"github.com/haaag/gm/internal/repo"
	"github.com/haaag/gm/internal/sys"
	"github.com/haaag/gm/internal/sys/files"
	"github.com/haaag/gm/internal/sys/terminal"
)

var ErrNotDefaultDB = errors.New("not the default database")

var (
	Copy bool
	Open bool
	Tags []string
	QR   bool

	Menu   bool
	Edit   bool
	Head   int
	Remove bool
	Tail   int

	Field     string
	JSON      bool
	Oneline   bool
	Multiline bool
	WithColor string

	Force   bool
	Status  bool
	Verbose bool
)

func initConfig() {
	// set logging level
	handler.LoggingLevel(&Verbose)
	// set force
	handler.Force(&Force)
	// enable color
	config.App.Color = WithColor != "never" && !terminal.IsPiped()
	menu.WithColor(&config.App.Color)
	// set terminal defaults
	terminal.NoColor(&config.App.Color)
	terminal.LoadMaxWidth()
	// enable color output
	color.Enable(&config.App.Color)
	// load data home path for the app.
	dataHomePath, err := loadDataPath()
	if err != nil {
		sys.ErrAndExit(err)
	}
	// set app/database settings/paths
	config.App.Path.Data = dataHomePath
	Cfg = repo.NewSQLiteCfg(filepath.Join(dataHomePath, DBName))
	// load menu settings
	if err := menu.LoadConfig(); err != nil {
		log.Println("error loading config:", err)
	}
}

// init sets the config for the root command.
func init() {
	cobra.OnInitialize(initConfig)
	// global
	pf := rootCmd.PersistentFlags()
	pf.StringVarP(&DBName, "name", "n", config.DefaultDBName, "database name")
	pf.StringVar(&WithColor, "color", "always", "output with pretty colors [always|never]")
	pf.BoolVarP(&Verbose, "verbose", "v", false, "verbose mode")
	pf.BoolVar(&Force, "force", false, "force action | don't ask confirmation")
	_ = pf.MarkHidden("help")
	// local
	f := rootCmd.Flags()
	// prints
	f.BoolVarP(&JSON, "json", "j", false, "output in JSON format")
	f.BoolVarP(&Multiline, "multiline", "M", false, "output in formatted multiline (fzf)")
	f.BoolVarP(&Oneline, "oneline", "O", false, "output in formatted oneline (fzf)")
	f.StringVarP(&Field, "field", "f", "", "output by field [id|url|title|tags]")
	// actions
	f.BoolVarP(&Copy, "copy", "c", false, "copy bookmark to clipboard")
	f.BoolVarP(&Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&QR, "qr", "q", false, "generate qr-code")
	f.BoolVarP(&Remove, "remove", "r", false, "remove a bookmarks by query or id")
	f.StringSliceVarP(&Tags, "tag", "t", nil, "list by tag")
	// experimental
	f.BoolVarP(&Menu, "menu", "m", false, "menu mode (fzf)")
	f.BoolVarP(&Edit, "edit", "e", false, "edit with preferred text editor")
	f.BoolVarP(&Status, "status", "s", false, "check bookmarks status")
	// modifiers
	f.IntVarP(&Head, "head", "H", 0, "the <int> first part of bookmarks")
	f.IntVarP(&Tail, "tail", "T", 0, "the <int> last part of bookmarks")
	// cmd settings
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
	rootCmd.SilenceErrors = true
	rootCmd.DisableSuggestions = true
	rootCmd.SuggestionsMinimumDistance = 1
	rootCmd.AddCommand(initCmd)
}

// createPaths creates the paths for the application.
func createPaths(t *terminal.Term, path string) error {
	if files.Exists(path) {
		return nil
	}
	f := frame.New(frame.WithColorBorder(color.Gray), frame.WithNoNewLine())
	f.Header(prettyVersion()).Ln().Row().Ln()
	f.Mid(format.PaddedLine("create path:", "'"+path+"'\n"))
	f.Mid(format.PaddedLine("create db:", "'"+Cfg.Fullpath()+"'\n"))
	f.Row("\n").Render()
	lines := format.CountLines(f.String())
	if !t.Confirm(f.Clean().Footer("continue?").String(), "y") {
		return handler.ErrActionAborted
	}
	// clean terminal keeping header+row
	headerN := 3
	lines += format.CountLines(f.String()) - headerN
	t.ClearLine(lines)
	if err := files.MkdirAll(path); err != nil {
		sys.ErrAndExit(err)
	}
	f.Clean()
	f.Success(fmt.Sprintf("Successfully created directory path '%s'.\n", path))
	f.Success("Successfully created initial bookmark.\n").Row("\n").Render()

	return nil
}

// isSubCmdCalled checks if a specific subcommand was invoked.
func isSubCmdCalled(cmd *cobra.Command, names ...string) bool {
	targetCmd, _, _ := cmd.Root().Find(os.Args[1:])
	for _, name := range names {
		if targetCmd != nil && targetCmd.Name() == name {
			log.Printf("isSubCmdCalled: '%s' is called\n", name)
			return true
		}
	}

	return false
}

// loadDataPath loads the path to the application's home directory.
//
// If environment variable GOMARKS_HOME is not set, uses the data user
// directory.
func loadDataPath() (string, error) {
	envDataHome := sys.Env(config.App.Env.Home, "")
	if envDataHome != "" {
		log.Printf("loadPath: envDataHome: %v\n", envDataHome)

		return config.PathJoin(envDataHome), nil
	}
	dataHome, err := config.DataPath()
	if err != nil {
		return "", fmt.Errorf("loading paths: %w", err)
	}
	log.Printf("loadPath: dataHome: %v\n", dataHome)

	return dataHome, nil
}

var initCmd = &cobra.Command{
	Use:    "init",
	Short:  "Initialize a new bookmarks database",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		// create paths for the application.
		t := terminal.New()
		if err := createPaths(t, config.App.Path.Data); err != nil {
			return err
		}
		// init database
		r, err := repo.New(Cfg)
		if r == nil {
			return fmt.Errorf("init database: %w", err)
		}
		defer r.Close()
		// initialize database
		if r.IsInitialized() && !Force {
			return fmt.Errorf("'%s' %w", r.Cfg.Name, repo.ErrDBAlreadyInitialized)
		}
		if err := r.Init(); err != nil {
			return fmt.Errorf("initializing database: %w", err)
		}
		f := frame.New(frame.WithColorBorder(color.Gray))
		// ignore initial bookmark if not DefaultDBName
		if Cfg.Name != config.DefaultDBName {
			s := color.Gray(Cfg.Name).Italic().String()
			success := color.BrightGreen("Successfully").Italic().String()
			f.Success(success + " initialized database " + s).Render()

			return nil
		}
		// initial bookmark
		ib := bookmark.New()
		ib.URL = config.App.Info.URL
		ib.Title = config.App.Info.Title
		ib.Tags = bookmark.ParseTags(config.App.Info.Tags)
		ib.Desc = config.App.Info.Desc
		// insert new bookmark
		if err := r.Insert(ib); err != nil {
			return fmt.Errorf("%w", err)
		}
		// print new record
		fmt.Print(bookmark.Frame(ib))
		s := color.BrightGreen("Successfully").Italic().String()
		f.Row().Success(s + " initialized database " + color.Gray(Cfg.Name).Italic().String()).
			Render()

		return nil
	},
}
