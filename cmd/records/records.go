// Package records provides Cobra subcommands for managing bookmarks and related
// entities, including record queries, actions, and tag operations.
package records

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/printer"
	"github.com/mateconpizza/gm/pkg/bookmark"
	"github.com/mateconpizza/gm/pkg/db"
)

// NewCmd is the root "records" command.
// It provides entrypoints for listing, filtering, and operating on bookmarks.
func NewCmd(cfg *config.Config) *cobra.Command {
	records := &cobra.Command{
		Use:     "rec",
		Aliases: []string{"r", "records"},
		Short:   "Records management",
		RunE:    Cmd,
	}

	InitFlags(records, cfg)

	return records
}

// Cmd is the main command and entrypoint.
func Cmd(cmd *cobra.Command, args []string) error {
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
		app.WithConfig(cfg),
		app.WithDB(r),
		app.WithConsole(ui.NewDefaultConsole(cmd.Context(), func(err error) {
			r.Close()
			sys.ErrAndExit(err)
		})),
	)

	m := handler.MenuMainForRecords[bookmark.Bookmark](cfg)
	bs, err := handler.Data(a, m, args)
	if err != nil {
		return err
	}
	if len(bs) == 0 {
		return db.ErrRecordNotFound
	}

	return exec(a, bs)
}

// InitFlags initializes CLI flags for the records command.
func InitFlags(cmd *cobra.Command, cfg *config.Config) {
	flag := cfg.Flags
	f := cmd.Flags()
	f.SortFlags = false

	// Actions
	f.BoolVarP(&flag.Open, "open", "o", false, "open bookmark in default browser")
	f.BoolVarP(&flag.Edit, "edit", "e", false, "edit bookmark with preferred text editor")
	f.BoolVarP(&flag.Remove, "remove", "r", false, "remove bookmark by query or ID")
	f.BoolVarP(&flag.Copy, "copy", "c", false, "copy bookmark URL to clipboard")
	f.BoolVarP(&flag.QR, "qr", "q", false, "generate QR code for bookmark URL")
	f.BoolVarP(&flag.Notes, "notes", "N", false, "display bookmark notes")
	f.BoolVarP(&flag.Menu, "menu", "m", false, "interactive menu mode using fzf")
	f.BoolVar(&flag.Multiline, "multiline", false, "output in multiline format (fzf)")

	// Display
	f.StringVarP(&cfg.Flags.Format, "format", "f", "",
		fmt.Sprintf("output format [%s]", strings.Join(printer.ValidFormats, "|")))

	// Filters
	InitFilterFlags(cmd, cfg)
}

func InitFilterFlags(cmd *cobra.Command, cfg *config.Config) {
	f := cmd.Flags()
	f.StringSliceVarP(&cfg.Flags.Tags, "tag", "t", nil, "filter bookmarks by tag(s)")
	f.IntVarP(&cfg.Flags.Head, "head", "H", 0, "show first N bookmarks")
	f.IntVarP(&cfg.Flags.Tail, "tail", "T", 0, "show last N bookmarks")
}

// exec handles the bookmark actions and output selection according to the
// provided flags.
func exec(a *app.Context, bs []*bookmark.Bookmark) error {
	f := a.Cfg.Flags
	switch {
	case f.Remove:
		return handler.Remove(a, bs)
	case f.Export:
		return handler.Export(bs)
	case f.Edit:
		return handler.Edit(a, bs)
	case f.Copy:
		return handler.Copy(bs)
	case f.Open && !f.QR:
		return handler.Open(a, bs)
	}

	c := a.Console()
	switch {
	case f.Format != "":
		return printer.Display(c, f.Format, bs)
	case f.QR:
		return handler.QR(a.Context(), bs, f.Open, a.Cfg.Name)
	case f.Notes:
		return printer.Notes(c, bs)
	default:
		return printer.Records(c, bs)
	}
}
