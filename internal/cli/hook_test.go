package cli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/picker/menucfg"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui/formatter"
)

func testSetupAppInfo(t *testing.T, version string) *application.Information {
	t.Helper()
	return &application.Information{
		Version: version,
	}
}

func TestHookInjectApp(t *testing.T) {
	t.Parallel()

	t.Run("injects config into empty context", func(t *testing.T) {
		t.Parallel()

		app := application.New(testSetupAppInfo(t, "1.0.0"))
		if err := app.Setup(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Initialize()
		cmd := &cobra.Command{
			Use: "test",
		}

		// Ensure command has a context
		cmd.SetContext(t.Context())

		hook := HookInjectApp(app)

		err := hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		retrievedApp, err := application.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedApp != app {
			t.Errorf("expected same config instance, got different one")
		}
	})

	t.Run("handles nil context", func(t *testing.T) {
		t.Parallel()
		// Arrange
		app := application.New(testSetupAppInfo(t, "1.0.0"))
		if err := app.Setup(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Initialize()

		cmd := &cobra.Command{
			Use: "test",
		}

		hook := HookInjectApp(app)

		err := hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error with nil context, got: %v", err)
		}

		retrievedApp, err := application.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedApp != app {
			t.Errorf("expected same config instance, got different one")
		}
	})

	t.Run("does not overwrite existing config in context", func(t *testing.T) {
		t.Parallel()

		originalApp := application.New(testSetupAppInfo(t, "1.0.0"))
		if err := originalApp.Setup(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		originalApp.DBName = "original"
		originalApp.Initialize()

		newApp := application.New(testSetupAppInfo(t, "2.0.0"))
		if err := newApp.Setup(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		newApp.DBName = "new"
		newApp.Initialize()

		cmd := &cobra.Command{
			Use: "test",
		}

		// Pre-inject original config
		ctx, err := application.ToContext(t.Context(), originalApp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cmd.SetContext(ctx)

		hook := HookInjectApp(newApp)
		err = hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		retrievedApp, err := application.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedApp.DBName != "original.db" {
			t.Errorf("expected original config to remain, got DBName=%s", retrievedApp.DBName)
		}

		if retrievedApp == newApp {
			t.Error("config was overwritten, expected original to remain")
		}
	})
}

func TestChainHooks(t *testing.T) {
	t.Parallel()
	t.Run("executes hooks in order", func(t *testing.T) {
		t.Parallel()

		var executed []string
		hook1 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook1")
			return nil
		}

		hook2 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook2")
			return nil
		}

		hook3 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook3")
			return nil
		}

		chained := ChainHooks(hook1, hook2, hook3)
		cmd := &cobra.Command{Use: "test"}

		err := chained(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(executed) != 3 {
			t.Fatalf("expected 3 hooks executed, got %d", len(executed))
		}

		if executed[0] != "hook1" || executed[1] != "hook2" || executed[2] != "hook3" {
			t.Errorf("hooks executed in wrong order: %v", executed)
		}
	})

	t.Run("stops on first error", func(t *testing.T) {
		t.Parallel()

		var executed []string
		testErr := errors.New("hook2 error")

		hook1 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook1")
			return nil
		}

		hook2 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook2")
			return testErr
		}

		hook3 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook3")
			return nil
		}

		chained := ChainHooks(hook1, hook2, hook3)
		cmd := &cobra.Command{Use: "test"}

		err := chained(cmd, []string{})
		if !errors.Is(err, testErr) {
			t.Fatalf("expected testErr, got: %v", err)
		}

		if len(executed) != 2 {
			t.Fatalf("expected 2 hooks executed before error, got %d", len(executed))
		}

		if executed[0] != "hook1" || executed[1] != "hook2" {
			t.Errorf("unexpected execution order: %v", executed)
		}
	})

	t.Run("skips nil hooks", func(t *testing.T) {
		t.Parallel()
		var executed []string

		hook1 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook1")
			return nil
		}

		hook2 := func(cmd *cobra.Command, args []string) error {
			executed = append(executed, "hook2")
			return nil
		}

		chained := ChainHooks(hook1, nil, hook2, nil)
		cmd := &cobra.Command{Use: "test"}

		err := chained(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		if len(executed) != 2 {
			t.Fatalf("expected 2 hooks executed (nils skipped), got %d", len(executed))
		}

		if executed[0] != "hook1" || executed[1] != "hook2" {
			t.Errorf("unexpected execution: %v", executed)
		}
	})

	t.Run("empty chain returns no error", func(t *testing.T) {
		t.Parallel()
		chained := ChainHooks()
		cmd := &cobra.Command{Use: "test"}

		err := chained(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error from empty chain, got: %v", err)
		}
	})
}

func TestHookGitSync(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setupCmd func() *cobra.Command
		wantErr  error
	}{
		{
			name: "skip_git_sync_on_current_command",
			setupCmd: func() *cobra.Command {
				return &cobra.Command{
					Use:         "sync",
					Annotations: SkipGitSync,
				}
			},
			wantErr: nil,
		},
		{
			name: "skip_git_sync_on_parent_command",
			setupCmd: func() *cobra.Command {
				parent := &cobra.Command{
					Use:         "parent",
					Annotations: SkipGitSync,
					RunE: func(cmd *cobra.Command, args []string) error {
						return nil
					},
				}

				child := &cobra.Command{
					Use: "child",
					RunE: func(cmd *cobra.Command, args []string) error {
						return nil
					},
				}

				parent.AddCommand(child)

				return child
			},
			wantErr: nil,
		},
		{
			name: "context_canceled",
			setupCmd: func() *cobra.Command {
				return &cobra.Command{}
			},
			wantErr: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := tt.setupCmd()

			ctx := t.Context()
			if errors.Is(tt.wantErr, context.Canceled) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			cmd.SetContext(ctx)

			if cmd == nil {
				defer func() {
					if r := recover(); r == nil {
						t.Fatalf("expected panic for nil command")
					}
				}()

				app := testutil.NewApp(t)

				_ = HookGitSync(app)(nil, nil)
				return
			}

			app := testutil.NewApp(t)
			err := HookGitSync(app)(cmd, nil)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestHookFormatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		menuMode        bool
		outputFlagValue string
		outputFlagSet   bool
		initialMenuFmt  string
		skipOnCmd       bool
		skipOnParent    bool

		wantMenuFormat string
		wantErr        error
	}{
		{
			name:            "normal_menu_with_output_override",
			menuMode:        true,
			outputFlagValue: "oneline",
			outputFlagSet:   true,
			initialMenuFmt:  "brief",
			wantMenuFormat:  "oneline",
		},
		{
			name:            "output_flag_not_set_keeps_menu_format",
			menuMode:        true,
			outputFlagValue: "",
			outputFlagSet:   false,
			initialMenuFmt:  "brief",
			wantMenuFormat:  "brief",
		},
		{
			name:            "menu_mode_false_ignores_output_flag",
			menuMode:        false,
			outputFlagValue: "oneline",
			outputFlagSet:   true,
			initialMenuFmt:  "brief",
			wantMenuFormat:  "brief",
		},
		{
			name:           "empty_initial_menu_format_errors",
			menuMode:       false,
			outputFlagSet:  false,
			initialMenuFmt: "",
			wantMenuFormat: "",
		},
		{
			name:            "invalid_output_format_errors",
			menuMode:        true,
			outputFlagValue: "not-a-real-format",
			outputFlagSet:   true,
			initialMenuFmt:  "brief",
			wantMenuFormat:  "not-a-real-format",
			wantErr:         formatter.ErrUnknownFormatter,
		},
		{
			name:           "skip_annotation_on_command_itself",
			menuMode:       true,
			outputFlagSet:  true,
			initialMenuFmt: "brief",
			skipOnCmd:      true,
			wantMenuFormat: "brief",
		},
		{
			name:           "skip_annotation_on_parent_command",
			menuMode:       true,
			outputFlagSet:  true,
			initialMenuFmt: "brief",
			skipOnParent:   true,
			wantMenuFormat: "brief",
		},
		{
			name:            "boundary_single_char_format",
			menuMode:        true,
			outputFlagValue: "x",
			outputFlagSet:   true,
			initialMenuFmt:  "brief",
			wantMenuFormat:  "x",
			wantErr:         formatter.ErrUnknownFormatter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testutil.NewApp(t)
			app.Menu = &menucfg.Config{Format: tt.initialMenuFmt}
			app.Flags.Menu = tt.menuMode
			app.Flags.Output = tt.outputFlagValue

			parent := &cobra.Command{Use: "root"}
			cmd := &cobra.Command{Use: "sub"}
			parent.AddCommand(cmd)

			cmd.Flags().String("output", "", "output format")
			if tt.outputFlagSet {
				if err := cmd.Flags().Set("output", tt.outputFlagValue); err != nil {
					t.Fatalf("setup: failed to set output flag: %v", err)
				}
			}

			if tt.skipOnCmd {
				cmd.Annotations = SkipFormatter
			}
			if tt.skipOnParent {
				parent.Annotations = SkipFormatter
			}

			hook := HookFormatter(app)
			err := hook(cmd, nil)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("HookFormatter() expected error, got nil")
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("HookFormatter() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("HookFormatter() unexpected error: %v", err)
			}

			if app.Menu.Format != tt.wantMenuFormat {
				t.Fatalf("app.Menu.Format = %q; want %q", app.Menu.Format, tt.wantMenuFormat)
			}
		})
	}
}

func TestHookEnsureAppInitialize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string

		cmdName     string
		unlockFlag  bool
		homeExists  bool
		mainDBExist bool

		wantErr error
	}{
		{
			name:       "dispatch_skips_init_command",
			cmdName:    "init",
			homeExists: false, // would otherwise error, but dispatch returns early
			wantErr:    nil,
		},
		{
			name:        "dispatch_skips_unlock_flag",
			cmdName:     "list",
			unlockFlag:  true,
			homeExists:  false, // would otherwise error, but dispatch returns early
			mainDBExist: false,
			wantErr:     nil,
		},
		{
			name:        "home_missing_not_initialized",
			cmdName:     "list",
			homeExists:  false,
			mainDBExist: false,
			wantErr:     application.ErrAppNoSetup,
		},
		{
			name:        "home_exists_main_db_missing",
			cmdName:     "list",
			homeExists:  true,
			mainDBExist: false,
			wantErr:     application.ErrMainNotFound,
		},
		{
			name:        "home_exists_main_db_exists_success",
			cmdName:     "list",
			homeExists:  true,
			mainDBExist: true,
			wantErr:     nil,
		},
		{
			name:        "unlock_flag_false_still_checks",
			cmdName:     "list",
			unlockFlag:  false,
			homeExists:  false,
			mainDBExist: false,
			wantErr:     application.ErrAppNoSetup,
		},
		{
			name:        "non_init_command_name_boundary",
			cmdName:     "initialize", // close to "init" but not an exact match
			homeExists:  false,
			mainDBExist: false,
			wantErr:     application.ErrAppNoSetup,
		},
		{
			name:        "home_and_db_both_exist_nested_path",
			cmdName:     "db",
			homeExists:  true,
			mainDBExist: true,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := testutil.NewApp(t)

			dir := t.TempDir()
			dataDir := filepath.Join(dir, "data")

			if tt.homeExists {
				testutil.NewDir(t, dataDir)
			}

			if tt.mainDBExist {
				testutil.NewDir(t, dataDir)
				testutil.NewFile(t, dataDir, application.MainDBName, []byte{})
			}

			app.Path.Data = dataDir

			root := &cobra.Command{Use: app.Cmd}
			cmd := &cobra.Command{Use: tt.cmdName}
			root.AddCommand(cmd)

			cmd.Flags().Bool("unlock", false, "unlock")
			if tt.unlockFlag {
				if err := cmd.Flags().Set("unlock", "true"); err != nil {
					t.Fatalf("setup: failed to set unlock flag: %v", err)
				}
			}

			hook := HookEnsureAppInitialize(app)
			err := hook(cmd, nil)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("HookEnsureAppInitialize() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("HookEnsureAppInitialize() error = %v; want errors.Is match for %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("HookEnsureAppInitialize() unexpected error: %v", err)
			}
		})
	}
}
