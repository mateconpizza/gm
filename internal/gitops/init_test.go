package gitops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mateconpizza/gm/internal/locker/gpg"
)

func TestInitGPG(t *testing.T) {
	t.Parallel()

	const fp = "ABCD1234EF567890ABCD1234EF567890ABCD1234"

	tests := []struct {
		name      string
		sub       string // path under the temp dir ("" = the temp dir itself)
		emptyPath bool   // call initGPG with path == ""
		gitAttr   string // gitAttrFile, relative to path
		fp        string
		setup     func(t *testing.T, path string) // optional pre-existing state
		wantFail  bool
		wantErr   error  // optional sentinel; checked with errors.Is
		wantMsg   string // optional substring of the error message
	}{
		{
			name:    "writes_gpg_id_and_gitattributes",
			gitAttr: ".gitattributes",
			fp:      fp,
		},
		{
			name:    "creates_nested_path",
			sub:     "a/b/c",
			gitAttr: ".gitattributes",
			fp:      fp,
		},
		{
			name:    "existing_files_are_overwritten",
			sub:     "repo",
			gitAttr: ".gitattributes",
			fp:      fp,
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{gpg.FingerprintIDFilename, ".gitattributes"} {
					if err := os.WriteFile(filepath.Join(path, name), []byte("stale"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			},
		},
		{
			name:    "empty_fingerprint_writes_bare_newline",
			gitAttr: ".gitattributes",
			fp:      "",
		},
		{
			name:      "empty_path",
			emptyPath: true,
			gitAttr:   ".gitattributes",
			fp:        fp,
			wantFail:  true,
			wantErr:   fs.ErrNotExist,
		},
		{
			name:    "path_is_a_file",
			sub:     "file",
			gitAttr: ".gitattributes",
			fp:      fp,
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			wantFail: true,
			wantErr:  syscall.ENOTDIR,
		},
		{
			name:    "gpg_id_target_is_a_directory",
			sub:     "repo",
			gitAttr: ".gitattributes",
			fp:      fp,
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Join(path, gpg.FingerprintIDFilename), 0o700); err != nil {
					t.Fatal(err)
				}
			},
			wantFail: true,
			wantErr:  syscall.EISDIR,
			wantMsg:  "failed to write .gpg-id",
		},
		{
			name:     "gitattributes_in_missing_subdir",
			gitAttr:  "missing/.gitattributes",
			fp:       fp,
			wantFail: true,
			wantErr:  fs.ErrNotExist,
			wantMsg:  "failed to write .gitattributes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), tt.sub)
			if tt.emptyPath {
				path = ""
			}
			if tt.setup != nil {
				tt.setup(t, path)
			}

			err := initGPG(path, tt.gitAttr, &gpg.Fingerprint{Fingerprint: tt.fp})

			if tt.wantFail {
				if err == nil {
					t.Fatal("initGPG() expected error, got nil")
				}
				if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
					t.Fatalf("initGPG() error = %v; want errors.Is %v", err, tt.wantErr)
				}
				if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
					t.Fatalf("initGPG() error = %q; want it to contain %q", err.Error(), tt.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("initGPG() unexpected error: %v", err)
			}

			gotID, readErr := os.ReadFile(filepath.Join(path, gpg.FingerprintIDFilename))
			if readErr != nil {
				t.Fatalf("read .gpg-id: %v", readErr)
			}
			if want := tt.fp + "\n"; string(gotID) != want {
				t.Fatalf(".gpg-id = %q; want %q", gotID, want)
			}

			gotAttr, readErr := os.ReadFile(filepath.Join(path, tt.gitAttr))
			if readErr != nil {
				t.Fatalf("read %s: %v", tt.gitAttr, readErr)
			}
			if string(gotAttr) != gpgGitAttribute {
				t.Fatalf("%s = %q; want %q", tt.gitAttr, gotAttr, gpgGitAttribute)
			}
		})
	}
}
