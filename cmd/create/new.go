package create

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd"
	dbCmd "github.com/mateconpizza/gm/cmd/db"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/repository"
)

func init() {
	newBookmarkCmd.Flags().StringVarP(&newRecordF.title, "title", "t", "", "bookmark title")
	newBookmarkCmd.Flags().StringVarP(&newRecordF.tags, "tags", "T", "", "bookmark tags")
	newCmd.AddCommand(newBookmarkCmd)

	newDatabaseCmd.Flags().
		StringVarP(&config.App.DBName, "name", "n", config.MainDBName, "new database name")
	_ = newDatabaseCmd.MarkFlagRequired("name")
	newCmd.AddCommand(newDatabaseCmd, newBackupCmd)

	cmd.Root.AddCommand(newCmd)
}

type newRecordType struct {
	title string
	tags  string
}

var (
	newRecordF = &newRecordType{}

	// newCmd represents the new command.
	newCmd = &cobra.Command{
		Use:   "new",
		Short: "New bookmark, database, backup",
		Example: `  gm new db -n newDBName
  gm new r --title='Some title' --tags='tag1 tag2'
  gm new bk`,
		RunE: newBookmarkCmd.RunE,
	}

	// newDatabaseCmd creates a new database.
	newDatabaseCmd = &cobra.Command{
		Use:         "database",
		Short:       "Create a new bookmarks database",
		Aliases:     []string{"db", "d"},
		Annotations: cmd.SkipDBCheckAnnotation,
		RunE:        dbCmd.DatabaseNewCmd.RunE,
		PostRunE:    dbCmd.DatabaseNewCmd.PostRunE,
	}

	// newBackupCmd creates a new backup.
	newBackupCmd = &cobra.Command{
		Use:     "backup",
		Short:   dbCmd.BackupNewCmd.Short,
		Aliases: []string{"bk"},
		RunE:    dbCmd.BackupNewCmd.RunE,
	}

	newBookmarkCmd = &cobra.Command{
		Use:     "record",
		Short:   "Create a new bookmark",
		Aliases: []string{"r"},
		RunE:    newBookmarkFunc,
	}
)

// newBookmarkCmd creates a new bookmark.
func newBookmarkFunc(command *cobra.Command, args []string) error {
	r, err := repository.New(config.App.DBPath)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	defer r.Close()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.Gray))),
		ui.WithTerminal(terminal.New(terminal.WithInterruptFn(func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		}))),
	)

	cgi := func(s string) string { return color.BrightGray(s).Italic().String() }
	cy := func(s string) string { return color.BrightYellow(s).String() }
	c.F.Headerln(cy("Add Bookmark" + cgi(" (ctrl+c to exit)"))).Rowln().Flush()

	b := bookmark.New()
	if err := handler.NewBookmark(c, r, b, newRecordF.title, newRecordF.tags, args); err != nil {
		return fmt.Errorf("%w", err)
	}

	if err := bookmark.Validate(b); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	if err := handler.SaveNewBookmark(c, r, b, config.App.Flags.Force); err != nil {
		return err
	}

	gr, err := git.NewRepo(config.App.DBPath)
	if err != nil {
		return err
	}

	if gr.IsTracked() {
		if err := gr.Add([]*bookmark.Bookmark{b}); err != nil {
			return err
		}
		if err := gr.RepoStatsWrite(); err != nil {
			return err
		}
		if err := gr.Commit("new bookmark"); err != nil {
			return err
		}
	}

	fmt.Print(c.SuccessMesg("bookmark added\n"))

	return nil
}
