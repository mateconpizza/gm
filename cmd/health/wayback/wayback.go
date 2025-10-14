package wayback

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/menu"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/scraper/wayback"
)

func NewCmd() *cobra.Command {
	cfg := config.New()
	waybackCmd := &cobra.Command{
		Use:     "wayback",
		Aliases: []string{"w", "wm", "wb"},
		Short:   "Wayback Machine",
		RunE:    waybackFunc,
	}

	f := waybackCmd.Flags()
	f.SortFlags = false
	f.BoolVarP(&cfg.Flags.Snapshot, "latest", "c", false,
		"fetches lastets snapshot from Wayback Machine")
	f.IntVarP(&cfg.Flags.Limit, "limit", "L", 0,
		"limit the number of snapshots returned")
	f.IntVarP(&cfg.Flags.Year, "year", "Y", 0,
		"fetches the last N snapshots from a specific year")
	f.BoolVarP(&cfg.Flags.Menu, "menu", "m", false,
		"interactive menu mode using fzf (select bookmarks)")
	f.BoolVar(&cfg.Flags.Multiline, "multiline", false,
		"output in multiline format (fzf)")

	return waybackCmd
}

func waybackFunc(cmd *cobra.Command, args []string) error {
	cfg := config.New()
	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)

	bs, err := handler.Data(menuForRecords(cfg), r, args, cfg.Flags)
	if err != nil {
		return err
	}

	n := len(bs)
	if n == 0 {
		return db.ErrRecordNotFound
	}

	f := cfg.Flags
	if n > wayback.MaxItems && !f.Force {
		return fmt.Errorf("%w: %d", wayback.ErrTooManyRecords, n)
	}

	c := ui.NewDefaultConsole(cmd.Context(), func(err error) {
		r.Close()
		sys.ErrAndExit(err)
	})

	switch {
	case f.Snapshot:
		return handler.WaybackLatestSnapshot(c, r, bs)
	case f.Limit > 0:
		return handler.WaybackSnapshots(c, r, bs)
	}

	return cmd.Help()
}

func menuForRecords[T bookmark.Bookmark](cfg *config.Config) *menu.Menu[T] {
	return menu.New[T](
		menu.WithSettings(cfg.Menu.Settings),
		menu.WithPreview(cfg.Cmd+" --name "+cfg.DBName+" records {1}"),
	)
}
