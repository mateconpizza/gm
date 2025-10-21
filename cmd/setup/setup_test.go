//nolint:paralleltest //unnecessary
package setup

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mateconpizza/gm/internal/app"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/db"
)

func setup(t *testing.T) *app.Context {
	t.Helper()

	cfg := &config.Config{
		Name:   config.AppName,
		Cmd:    config.AppCommand,
		DBName: config.MainDBName,
		Path:   &config.Path{},
		Flags: &config.Flags{
			ColorStr: "never",
			Color:    false,
		},
		Info: &config.Information{
			URL:     "https://github.com/mateconpizza/gm#readme",
			Title:   "Gomarks: A bookmark manager",
			Tags:    "golang,awesome,bookmarks,cli",
			Desc:    "Simple yet powerful bookmark manager for your terminal",
			Version: "0.0.1",
		},
		Env: &config.Env{
			Home:   "GOMARKS_HOME",
			Editor: "GOMARKS_EDITOR",
		},
	}

	temp := t.TempDir()
	t.Setenv(cfg.Env.Home, temp)

	cfg.DBPath = filepath.Join(temp, cfg.DBName)
	cfg.Path.Data = temp

	tm := terminal.New(
		terminal.WithContext(t.Context()),
		terminal.WithWriter(io.Discard),
	)

	return app.New(t.Context(),
		app.WithConfig(cfg),
		app.WithConsole(ui.NewConsole(
			ui.WithTerminal(tm),
			ui.WithFrame(frame.New()),
		)),
	)
}

func TestSuccessfulInitializationWithMainDatabase(t *testing.T) {
	a := setup(t)
	var buf bytes.Buffer
	a.SetWriter(&buf)

	err := initializeAction(a)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "initialized database") {
		t.Errorf("expected output to contain 'initialized database', got %q", output)
	}
	if !strings.Contains(output, a.Cfg.Info.Title) {
		t.Errorf("expected output to contain title %q, got %q", a.Cfg.Info.Title, output)
	}

	// Verify database was actually initialized
	store, err := db.New(a.Cfg.DBPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	if !store.IsInitialized(t.Context()) {
		t.Error("expected database to be initialized")
	}

	// Verify initial bookmark was inserted
	bm, err := store.ByID(a.Ctx, 1)
	if err != nil {
		t.Fatalf("failed to get bookmark: %v", err)
	}
	if bm.URL != a.Cfg.Info.URL {
		t.Errorf("expected URL %q, got %q", a.Cfg.Info.URL, bm.URL)
	}
	if bm.Title != a.Cfg.Info.Title {
		t.Errorf("expected title %q, got %q", a.Cfg.Info.Title, bm.Title)
	}
}

func TestSuccessfulInitializationWithNonMainDatabase(t *testing.T) {
	a := setup(t)
	a.Cfg.DBName = "test-db"
	a.Cfg.DBPath = filepath.Join(a.Cfg.Path.Data, a.Cfg.DBName)
	var buf bytes.Buffer
	a.SetWriter(&buf)

	err := initializeAction(a)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "initialized database test-db") {
		t.Errorf("expected output to contain 'initialized database test-db', got %q", output)
	}
	// Should not contain bookmark frame for non-main DB
	if strings.Contains(output, a.Cfg.Info.Title) {
		t.Errorf("expected output to not contain title for non-main DB, got %q", output)
	}
}

func TestFailsWhenDatabaseAlreadyInitializedWithoutForceFlag(t *testing.T) {
	t.Skip("Update database initialization")
	a := setup(t)

	// Initialize database first time
	err := initializeAction(a)
	if err != nil {
		t.Fatalf("first initialization failed: %v", err)
	}

	// Try to initialize again
	err = initializeAction(a)
	if err == nil {
		t.Fatal("expected error when reinitializing without force flag")
	}
	if !errors.Is(err, db.ErrDBAlreadyInitialized) {
		t.Errorf("expected ErrDBAlreadyInitialized, got %v", err)
	}
}

func TestSucceedsWhenDatabaseAlreadyInitializedWithForceFlag(t *testing.T) {
	t.Skip("Update database initialization")
	a := setup(t)

	// Initialize database first time
	err := initializeAction(a)
	if err != nil {
		t.Fatalf("first initialization failed: %v", err)
	}

	// Set force flag and try again
	a.Cfg.Flags.Force = true
	err = initializeAction(a)
	if err != nil {
		t.Errorf("expected no error with force flag, got %v", err)
	}
}

func TestFailsWhenInitReturnsErr(t *testing.T) {
	a := setup(t)
	// Set invalid DB path
	a.Cfg.DBPath = "/invalid/path/\x00/db"

	err := initializeAction(a)

	if err == nil {
		t.Fatal("expected error with invalid DB path")
	}
}

func TestFailsWhenBookmarkInsertionFails(t *testing.T) {
	a := setup(t)
	// Set invalid bookmark data that would cause insertion to fail
	a.Cfg.Info.URL = "" // Invalid bookmark

	err := initializeAction(a)
	if err == nil {
		t.Fatal("expected error with invalid bookmark data")
	}
}

func TestParseAndStoreBookmarkTags(t *testing.T) {
	a := setup(t)
	var buf bytes.Buffer
	a.SetWriter(&buf)

	err := initializeAction(a)
	if err != nil {
		t.Fatalf("initialization failed: %v", err)
	}

	store, err := db.New(a.Cfg.DBPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer store.Close()

	bm, err := store.ByID(a.Ctx, 1)
	if err != nil {
		t.Fatalf("failed to get bookmark: %v", err)
	}

	gotTags := strings.FieldsFunc(bm.Tags, func(r rune) bool {
		return r == ',' || r == ' '
	})
	expectedTags := []string{"golang", "awesome", "bookmarks", "cli"}
	got := len(gotTags)
	want := len(expectedTags)
	if got != want {
		t.Errorf("expected %d tags, got %v", want, gotTags)
	}

	for _, tag := range expectedTags {
		found := false
		for bmTag := range strings.SplitSeq(bm.Tags, ",") {
			if bmTag == "" {
				continue
			}
			if bmTag == tag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tag %q not found in %v", tag, bm.Tags)
		}
	}
}
