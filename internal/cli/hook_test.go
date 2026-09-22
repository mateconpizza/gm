package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/testutil"
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
