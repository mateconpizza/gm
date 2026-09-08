package dbops

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	files "github.com/mateconpizza/gofiles"

	"github.com/mateconpizza/gm/internal/application"
	"github.com/mateconpizza/gm/internal/deps"
	"github.com/mateconpizza/gm/internal/locker"
	"github.com/mateconpizza/gm/internal/sys"
	"github.com/mateconpizza/gm/internal/testutil"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/pkg/ansi"
	"github.com/mateconpizza/gm/pkg/db"
)

func TestRemoveRepo_Success(t *testing.T) {
	t.Parallel()
	ansi.DisableColor()

	t.Run("successfully remove main database", func(t *testing.T) {
		t.Parallel()
		d := testutil.NewDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Flags.Force = true
		r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
		d.WithRepo(r)
		var buf bytes.Buffer
		d.WithWriter(&buf)

		err = Remove(t.Context(), d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Successfully database main removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(app.Path.DB()) {
			t.Fatalf("file %q was not deleted", app.Path.DB())
		}
	})

	t.Run("successfully remove a database", func(t *testing.T) {
		t.Parallel()
		d := testutil.NewDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.DBName = "somedatabase.db"
		app.Path.Database = filepath.Join(app.Path.Data, app.DBName)
		app.Flags.Force = true
		r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
		d.WithRepo(r)
		var buf bytes.Buffer
		d.WithWriter(&buf)

		err = Remove(t.Context(), d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		name := files.StripExts(app.DBName)
		if !strings.Contains(output, "Successfully database "+name+" removed") {
			t.Fatalf("%v", output)
		}

		if files.Exists(app.Path.DB()) {
			t.Fatalf("file %q was not deleted", app.Path.DB())
		}
	})
}

func TestRemoveRepo_Fail(t *testing.T) {
	t.Parallel()

	t.Run("fails with database not found", func(t *testing.T) {
		t.Parallel()
		d := testutil.NewDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		app.Path.Database = filepath.Join(app.Path.Data, "nonexistent.db")

		err = Remove(t.Context(), d)
		if !errors.Is(err, db.ErrDBNotFound) {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails with main database cannot be removed without flag force", func(t *testing.T) {
		t.Parallel()
		d := testutil.NewDeps(t)
		app, err := d.Application(t.Context())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
		d.WithRepo(r)

		err = Remove(t.Context(), d)
		if !errors.Is(err, ErrInvalidOption) {
			t.Fatalf("unexpected error: %v", err)
		}

		gotOutput := err.Error()
		wantOutput := "removing the main database requires"
		if !strings.Contains(gotOutput, wantOutput) {
			t.Fatalf("want: %q, got: %q", wantOutput, gotOutput)
		}
	})
}

func TestNewBackup(t *testing.T) {
	t.Parallel()

	setupWithInput := func(t *testing.T, input string) *deps.Deps {
		t.Helper()

		temp := t.TempDir()
		app := testutil.NewApp(t).WithHomePath(temp)
		_ = app.SetDatabase(app.DBName)

		c := testutil.NewConsoleWithInput(t, input)
		d := deps.New(
			deps.WithApplication(app),
			deps.WithConsole(c),
		)

		r := testutil.NewInitializedEmptyDB(t, app.Path.DB())
		return d.WithRepo(r)
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T) *deps.Deps
		wantErr    error
		wantErrMsg string
	}{
		{
			name: "backup_with_yes_flag",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()

				d := testutil.NewDeps(t)
				app, _ := d.Application(t.Context())
				app.Flags.Yes = true

				r := testutil.NewInitializedDBWithBookmarks(
					t, app.Path.DB(), 5,
				)

				return d.WithRepo(r)
			},
		},
		{
			name: "backup_with_confirmation",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()

				d := setupWithInput(t, "y\n")
				app, _ := d.Application(t.Context())
				app.Flags.Yes = false

				return d
			},
		},
		{
			name: "abort_confirmation",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()

				d := setupWithInput(t, "n\n")
				app, _ := d.Application(t.Context())
				app.Flags.Yes = false

				return d
			},
			wantErr: sys.ErrExitFailure,
		},
		{
			name: "db_not_found",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				return testutil.NewDeps(t)
			},
			wantErr: db.ErrDBNotFound,
		},
		{
			name: "db_empty",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()

				d := testutil.NewDeps(t)
				app, _ := d.Application(t.Context())

				f, err := os.Create(app.Path.DB())
				if err != nil {
					t.Fatalf("failed to create empty DB: %v", err)
				}
				if err := f.Close(); err != nil {
					t.Fatalf("failed to close empty DB: %v", err)
				}

				return d
			},
			wantErr: db.ErrDBEmpty,
		},
		{
			name: "backup_directory_creation_fails",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()

				d := testutil.NewDeps(t)
				app, _ := d.Application(t.Context())
				app.Flags.Yes = true

				r := testutil.NewInitializedEmptyDB(t, app.Path.DB())

				if err := os.WriteFile(app.Path.Backup(), []byte("conflict"), 0o644); err != nil {
					t.Fatalf("failed to create conflict file: %v", err)
				}

				return d.WithRepo(r)
			},
			wantErrMsg: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := tt.setup(t)
			err := NewBackup(t.Context(), d)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("NewBackup() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("NewBackup() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("NewBackup() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("NewBackup() error = %q; want substring %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("NewBackup() unexpected error: %v", err)
			}
		})
	}
}

type fakeReorderStore struct {
	reorderIDs   func(context.Context) error
	backup       func(context.Context, string) (string, error)
	reorderCalls int
	backupCalls  int
}

func (f *fakeReorderStore) ReorderIDs(ctx context.Context) error {
	f.reorderCalls++
	if f.reorderIDs != nil {
		return f.reorderIDs(ctx)
	}
	return nil
}

func (f *fakeReorderStore) Backup(ctx context.Context, destRoot string) (string, error) {
	f.backupCalls++
	if f.backup != nil {
		return f.backup(ctx, destRoot)
	}
	return filepath.Join(destRoot, "backup.db"), nil
}

func TestReorderDatabase(t *testing.T) {
	t.Parallel()
	errBackup := errors.New("backup failed")
	errReorder := errors.New("reorder failed")

	tests := []struct {
		name         string
		input        string
		cancelCtx    bool
		setup        func(t *testing.T, app *application.App, r *fakeReorderStore)
		wantErr      error
		wantErrMsg   string
		backupCalls  int
		reorderCalls int
	}{
		{
			name:         "normal_reorder_with_backup",
			input:        "y\ny\n",
			reorderCalls: 1,
			backupCalls:  1,
		},
		{
			name:         "normal_reorder_without_backup",
			input:        "y\nn\n",
			reorderCalls: 1,
		},
		{
			name:    "abort_at_continue_prompt",
			input:   "n\n",
			wantErr: sys.ErrExitFailure,
		},
		{
			name:  "backup_error",
			input: "y\ny\n",
			setup: func(t *testing.T, app *application.App, r *fakeReorderStore) {
				t.Helper()
				r.backup = func(context.Context, string) (string, error) {
					return "", errBackup
				}
			},
			wantErr: errBackup,
		},
		{
			name:      "reorder_error",
			input:     "y\nn\n",
			cancelCtx: false,
			setup: func(t *testing.T, app *application.App, r *fakeReorderStore) {
				t.Helper()
				r.reorderIDs = func(context.Context) error {
					return errReorder
				}
			},
			wantErr: errReorder,
		},
		{
			name:  "reorder_context_canceled",
			input: "y\nn\n",
			setup: func(t *testing.T, app *application.App, r *fakeReorderStore) {
				t.Helper()
				r.reorderIDs = func(context.Context) error {
					return context.Canceled
				}
			},
			wantErr: context.Canceled,
		},
		{
			name:  "backup_directory_is_file",
			input: "y\ny\n",
			setup: func(t *testing.T, app *application.App, r *fakeReorderStore) {
				t.Helper()
				backupPath := app.Path.Backup()
				if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
					t.Fatalf("failed to create backup parent: %v", err)
				}
				if err := os.WriteFile(backupPath, []byte("conflict"), 0o644); err != nil {
					t.Fatalf("failed to create backup conflict: %v", err)
				}
			},
			wantErrMsg: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			app := application.NewApp(t.TempDir())

			if err := app.SetDatabase("test.db"); err != nil {
				t.Fatalf("SetDatabase() unexpected error: %v", err)
			}

			c := ui.NewConsole(ui.WithWriter(io.Discard))
			c.Frame().SetWriter(io.Discard)
			c.Term().SetReader(strings.NewReader(tt.input))

			r := &fakeReorderStore{}

			if tt.setup != nil {
				tt.setup(t, app, r)
			}

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			if tt.cancelCtx {
				cancel()
			}

			err := ReorderDatabase(ctx, app, r, c)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("ReorderDatabase() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ReorderDatabase() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("ReorderDatabase() expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Fatalf("ReorderDatabase() error = %q; want substring %q", err, tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("ReorderDatabase() unexpected error: %v", err)
			}
			if r.reorderCalls != tt.reorderCalls {
				t.Fatalf("ReorderIDs called %d times; want %d", r.reorderCalls, tt.reorderCalls)
			}
			if r.backupCalls != tt.backupCalls {
				t.Fatalf("Backup called %d times; want %d", r.backupCalls, tt.backupCalls)
			}
		})
	}
}

func TestDrop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		setup   func(t *testing.T) *deps.Deps
		wantErr error
	}{
		{
			name: "flag_yes_drops_without_confirm",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				tempDir := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(tempDir)
				app.SetDatabase(app.DBName)
				app.Flags.Yes = true

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsole(t, io.Discard)),
					deps.WithRepo(r),
				)
			},
			wantErr: nil,
		},
		{
			name: "flag_force_drops_without_confirm",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.SetDatabase(app.DBName)
				app.Flags.Force = true

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsole(t, io.Discard)),
					deps.WithRepo(r),
				)
			},
			wantErr: nil,
		},
		{
			name: "confirm_declined",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.SetDatabase(app.DBName)

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsoleWithInput(t, "n\n")),
					deps.WithRepo(r),
				)
			},
			wantErr: sys.ErrExitFailure,
		},
		{
			name: "confirm_accepted_explicit_yes",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.SetDatabase(app.DBName)

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsoleWithInput(t, "y\n")),
					deps.WithRepo(r),
				)
			},
			wantErr: nil,
		},
		{
			name: "confirm_accepted_default_no",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.SetDatabase(app.DBName)

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsoleWithInput(t, "\n")), // blank -> default "n"
					deps.WithRepo(r),
				)
			},
			wantErr: sys.ErrExitFailure,
		},
		{
			name: "dropping_main_database_warns_but_still_confirms",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.DBName = application.MainDBName
				app.SetDatabase(app.DBName)

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsoleWithInput(t, "y\n")),
					deps.WithRepo(r),
				)
			},
			wantErr: nil,
		},
		{
			name: "dropping_main_database_declined",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.DBName = application.MainDBName
				app.SetDatabase(app.DBName)

				r := testutil.NewInitializedEmptyDB(t, app.Path.Database)
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsoleWithInput(t, "n\n")),
					deps.WithRepo(r),
				)
			},
			wantErr: sys.ErrExitFailure,
		},
		{
			name: "repository_unavailable",
			setup: func(t *testing.T) *deps.Deps {
				t.Helper()
				temp := t.TempDir()
				app := testutil.NewApp(t).WithHomePath(temp)
				app.SetDatabase(app.DBName)

				// No repo registered: d.Repository() is expected to fail.
				return deps.New(
					deps.WithApplication(app),
					deps.WithConsole(testutil.NewConsole(t, io.Discard)),
				)
			},
			wantErr: db.ErrDBNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			d := tt.setup(t)
			err := Drop(t.Context(), d)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Drop() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Drop() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("Drop() unexpected error: %v", err)
			}
		})
	}
}

func TestLock(t *testing.T) {
	t.Parallel()

	newUnlockedFile := func(t *testing.T, dir, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("db-content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		return p
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) (c *ui.Console, items []string, wantLocked, wantUnlocked []string)
		wantErr error
	}{
		{
			name: "normal_locks_single_file",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f := newUnlockedFile(t, tempDir, "main.db")
				c := testutil.NewConsoleWithInput(t, "y\nsecret\nsecret\n")
				return c, []string{f}, []string{f}, nil
			},
			wantErr: nil,
		},
		{
			name: "already_locked_returns_error",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f := newUnlockedFile(t, tempDir, "main.db")
				// create the .enc counterpart so IsLocked reports it as locked.
				encPath := locker.Extension.Join(f)
				if err := os.WriteFile(encPath, []byte("locked"), 0o644); err != nil {
					t.Fatalf("failed to create locked marker file: %v", err)
				}
				c := testutil.NewConsoleWithInput(t, "")
				return c, []string{f}, nil, nil
			},
			wantErr: locker.ErrFileLocked,
		},
		{
			name: "file_does_not_exist",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				missing := filepath.Join(tempDir, "missing.db")
				c := testutil.NewConsoleWithInput(t, "\n\n")
				return c, []string{missing}, nil, nil
			},
			wantErr: os.ErrNotExist,
		},
		{
			name: "confirm_declined_skips_file",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f := newUnlockedFile(t, tempDir, "main.db")
				c := testutil.NewConsoleWithInput(t, "n\n")
				return c, []string{f}, nil, []string{f}
			},
			wantErr: nil,
		},
		{
			name: "password_mismatch",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f := newUnlockedFile(t, tempDir, "main.db")
				c := testutil.NewConsoleWithInput(t, "y\nsecret1\nsecret2\n")
				return c, []string{f}, nil, []string{f}
			},
			wantErr: locker.ErrPassphraseMismatch,
		},
		{
			name: "empty_items_returns_nil",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				c := testutil.NewConsoleWithInput(t, "")
				return c, []string{}, nil, nil
			},
			wantErr: nil,
		},
		{
			name: "multiple_items_first_declined_second_locked",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f1 := newUnlockedFile(t, tempDir, "one.db")
				f2 := newUnlockedFile(t, tempDir, "two.db")
				c := testutil.NewConsoleWithInput(t, "n\ny\nsecret\nsecret\n")
				return c, []string{f1, f2}, []string{f2}, []string{f1}
			},
			wantErr: nil,
		},
		{
			name: "multiple_items_all_locked",
			setup: func(t *testing.T) (*ui.Console, []string, []string, []string) {
				t.Helper()
				tempDir := t.TempDir()
				f1 := newUnlockedFile(t, tempDir, "one.db")
				f2 := newUnlockedFile(t, tempDir, "two.db")
				c := testutil.NewConsoleWithInput(t, "y\nsecretA\nsecretA\ny\nsecretB\nsecretB\n")
				return c, []string{f1, f2}, []string{f1, f2}, nil
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, items, wantLocked, wantUnlocked := tt.setup(t)
			err := Lock(t.Context(), c, items)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Lock() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Lock() expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Lock() unexpected error: %v", err)
			}

			for _, p := range wantLocked {
				encPath := locker.Extension.Join(p)
				if !files.Exists(encPath) {
					t.Errorf("expected %q to be locked (missing %q)", filepath.Base(p), encPath)
				}
			}
			for _, p := range wantUnlocked {
				encPath := locker.Extension.Join(p)
				if files.Exists(encPath) {
					t.Errorf("expected %q to remain unlocked, but found %q", filepath.Base(p), encPath)
				}
			}
		})
	}
}

func TestUnlock(t *testing.T) {
	t.Parallel()

	newLockedFile := func(t *testing.T, dir, name, pwd string) string {
		t.Helper()
		plain := filepath.Join(dir, name)
		if err := os.WriteFile(plain, []byte("db-content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		if err := locker.Lock(plain, pwd); err != nil {
			t.Fatalf("failed to lock fixture file: %v", err)
		}
		return plain // unlock takes the unextended path, same as Lock does
	}

	newUnlockedFile := func(t *testing.T, dir, name string) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("db-content"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		return p
	}

	tests := []struct {
		name       string
		setup      func(t *testing.T) (c consolePass, items []string)
		wantErr    error
		wantErrAny bool // expect a non-nil error whose exact sentinel is unspecified (e.g. from locker.Unlock)
	}{
		{
			name: "normal_unlocks_single_file",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p := newLockedFile(t, dir, "main.db", "secret")
				c := testutil.NewConsoleWithInput(t, "y\nsecret\n")
				return c, []string{p}
			},
			wantErr: nil,
		},
		{
			name: "already_unlocked_returns_error",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p := newUnlockedFile(t, dir, "main.db")
				c := testutil.NewConsoleWithInput(t, "")
				return c, []string{p}
			},
			wantErr: locker.ErrFileUnlocked,
		},
		{
			name: "confirm_declined",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p := newLockedFile(t, dir, "main.db", "secret")
				c := testutil.NewConsoleWithInput(t, "n\n")
				return c, []string{p}
			},
			wantErr: sys.ErrExitFailure,
		},
		{
			name: "confirm_accepted_default_yes",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p := newLockedFile(t, dir, "main.db", "secret")
				c := testutil.NewConsoleWithInput(t, "\nsecret\n") // blank -> default "y"
				return c, []string{p}
			},
			wantErr: nil,
		},
		{
			name: "wrong_password",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p := newLockedFile(t, dir, "main.db", "secret")
				c := testutil.NewConsoleWithInput(t, "y\nwrong-password\n")
				return c, []string{p}
			},
			wantErrAny: true,
		},
		{
			name: "empty_items_returns_nil",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				c := testutil.NewConsoleWithInput(t, "")
				return c, []string{}
			},
			wantErr: nil,
		},
		{
			name: "multiple_items_stops_on_first_error",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				alreadyUnlocked := newUnlockedFile(t, dir, "one.db")
				locked := newLockedFile(t, dir, "two.db", "secret")
				c := testutil.NewConsoleWithInput(t, "")
				return c, []string{alreadyUnlocked, locked}
			},
			wantErr: locker.ErrFileUnlocked,
		},
		{
			name: "multiple_items_all_unlocked",
			setup: func(t *testing.T) (consolePass, []string) {
				t.Helper()
				dir := t.TempDir()
				p1 := newLockedFile(t, dir, "one.db", "secretA")
				p2 := newLockedFile(t, dir, "two.db", "secretB")
				c := testutil.NewConsoleWithInput(t, "y\nsecretA\ny\nsecretB\n")
				return c, []string{p1, p2}
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, items := tt.setup(t)
			// Snapshot expected unlocked-state paths before mutation, since a
			// successful Unlock renames/removes the .enc file.
			wantUnlocked := make([]string, len(items))
			copy(wantUnlocked, items)

			err := Unlock(t.Context(), c, items)

			switch {
			case tt.wantErr != nil:
				if err == nil {
					t.Fatalf("Unlock() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Unlock() expected error %v, got %v", tt.wantErr, err)
				}
				return
			case tt.wantErrAny:
				if err == nil {
					t.Fatalf("Unlock() expected a non-nil error, got nil")
				}
				return
			default:
				if err != nil {
					t.Fatalf("Unlock() unexpected error: %v", err)
				}
			}

			for _, p := range wantUnlocked {
				if err := locker.IsLocked(p); err != nil {
					t.Errorf("expected %q to be unlocked, but IsLocked returned: %v", filepath.Base(p), err)
				}
			}
		})
	}
}
