package database

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/cmd/cmdutil"
	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/gitops"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
	"github.com/mateconpizza/gm/pkg/git"
)

func newBackupRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Short:   "remove one or more backups",
		Aliases: []string{"backup", "b", "backups"},
		RunE: func(cmd *cobra.Command, args []string) error {
			input := "s\n" // input for prompt, this will show the menu to select backups files.
			d := deps.New(
				deps.WithApplication(app),
				deps.WithConsole(ui.NewConsole(
					ui.WithFrame(frame.New(frame.WithColorBorder(ansi.Gray))),
					ui.WithTerminal(terminal.New(
						terminal.WithInterruptFn(func(err error) {
							db.Shutdown()
							sys.ErrAndExit(err)
						}),
						terminal.WithReader(strings.NewReader(input)),
						terminal.WithWriter(io.Discard), // send output to null, show no prompt
					)),
				)),
			)

			return handler.RemoveBackups(cmd.Context(), d)
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}

func newDatabaseRemoveCmd(app *application.App) *cobra.Command {
	c := &cobra.Command{
		Use:     "rm",
		Aliases: []string{"remove"},
		Short:   "remove a database",
		RunE: func(cmd *cobra.Command, args []string) error {
			d, cancel, err := cmdutil.SetupDeps(cmd, &args)
			if err != nil {
				return err
			}
			defer cancel()

			r, err := d.Repository()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			bs, err := r.All(ctx)
			if err != nil {
				return err
			}

			if err := handler.RemoveRepo(ctx, d); err != nil {
				return err
			}

			m, err := gitops.NewManager(app)
			if err != nil {
				return err
			}

			gr := gitops.NewRepo(m, r.Name(), git.WithRepoStore(r))
			if !m.IsTracked(gr.Name()) {
				return nil
			}

			var sb strings.Builder
			fmt.Fprintf(&sb, "[%s] removed and untracked", gr.Name())
			if len(bs) > 0 {
				fmt.Fprintf(&sb, " (-del:%d)", len(bs))
			}

			return m.Untrack(cmd.Context(), gr, sb.String())
		},
	}

	cmdutil.FlagDBRequired(c, app)

	return c
}
