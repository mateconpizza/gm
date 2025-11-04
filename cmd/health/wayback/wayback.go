package wayback

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
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

func NewCmd(cfg *config.Config) *cobra.Command {
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
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	r, err := db.New(cfg.DBPath)
	if err != nil {
		return err
	}
	defer r.Close()

	terminal.ReadPipedInput(&args)
	a := app.New(cmd.Context(),
		app.WithDB(r),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	m := handler.MenuSimple[bookmark.Bookmark](cfg, menu.WithMultiSelection(),
		menu.WithHeader("select record/s"))

	return processWayback(a, m, args)
}

func processWayback(a *app.Context, m *menu.Menu[bookmark.Bookmark], args []string) error {
	bs, err := handler.Data(a, m, args)
	if err != nil {
		return err
	}

	n := len(bs)
	if n == 0 {
		return db.ErrRecordNotFound
	}

	flags := a.Cfg.Flags
	if n > wayback.MaxItems && !flags.Force {
		return wayback.ErrTooManyRecords
	}

	f, p := a.Console().Frame(), a.Console().Palette()
	f.Headerln(p.BrightRed.With(p.Bold).Sprintf("Bookmarks (%d)", n)).Rowln()
	for i := range bs {
		if i >= wayback.MaxItems {
			f.Midln(p.BrightBlack.With(p.Italic).Sprintf("%d more ...", n-i)).Rowln()
			break
		}
		f.Midln(bs[i].URL)
	}
	f.Flush()

	if !a.Console().Confirm("continue?", "n") {
		return sys.ErrExitFailure
	}

	switch {
	case flags.Snapshot:
		return handler.WaybackLatestSnapshot(a, bs)
	case flags.Limit > 0:
		return handler.WaybackSnapshots(a, bs)
	}

	return nil
}
