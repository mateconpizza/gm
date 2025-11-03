//nolint:funlen,err113,gocyclo //testing
package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/config"
)

func TestHookInjectConfig(t *testing.T) {
	t.Parallel()

	t.Run("injects config into empty context", func(t *testing.T) {
		t.Parallel()

		cfg := config.NewDefaultConfig("1.0.0")
		cfg.Initialize()
		cmd := &cobra.Command{
			Use: "test",
		}

		// Ensure command has a context
		cmd.SetContext(context.Background())

		hook := HookInjectConfig(cfg)

		err := hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		retrievedCfg, err := config.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedCfg != cfg {
			t.Errorf("expected same config instance, got different one")
		}
	})

	t.Run("handles nil context", func(t *testing.T) {
		t.Parallel()
		// Arrange
		cfg := config.NewDefaultConfig("1.0.0")
		cfg.Initialize()

		cmd := &cobra.Command{
			Use: "test",
		}

		hook := HookInjectConfig(cfg)

		err := hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error with nil context, got: %v", err)
		}

		retrievedCfg, err := config.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedCfg != cfg {
			t.Errorf("expected same config instance, got different one")
		}
	})

	t.Run("does not overwrite existing config in context", func(t *testing.T) {
		t.Parallel()

		originalCfg := config.NewDefaultConfig("1.0.0")
		originalCfg.DBName = "original"
		originalCfg.Initialize()

		newCfg := config.NewDefaultConfig("2.0.0")
		newCfg.DBName = "new"
		newCfg.Initialize()

		cmd := &cobra.Command{
			Use: "test",
		}

		// Pre-inject original config
		cmd.SetContext(config.ToContext(context.Background(), originalCfg))

		hook := HookInjectConfig(newCfg)

		err := hook(cmd, []string{})
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}

		retrievedCfg, err := config.FromContext(cmd.Context())
		if err != nil {
			t.Fatalf("expected config in context, got error: %v", err)
		}

		if retrievedCfg.DBName != "original.db" {
			t.Errorf("expected original config to remain, got DBName=%s", retrievedCfg.DBName)
		}

		if retrievedCfg == newCfg {
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

		//nolint:gocritic //testing
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
