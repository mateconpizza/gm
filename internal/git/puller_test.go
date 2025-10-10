package git

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/sys/terminal"
	"github.com/mateconpizza/gm/internal/ui"
	"github.com/mateconpizza/gm/internal/ui/color"
	"github.com/mateconpizza/gm/internal/ui/frame"
	"github.com/mateconpizza/gm/pkg/files"
)

func setupRepoSummary(t *testing.T) *SyncGitSummary {
	t.Helper()

	return &SyncGitSummary{
		GitBranch:          "master",
		GitRemote:          "https://github.com/pepe/hongo.git",
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "SHA-256",

		RepoStats: &dbtask.RepoStats{
			Name:      "test_pepe",
			Bookmarks: 10,
			Tags:      20,
			Favorites: 5,
			Size:      "256",
		},

		ClientInfo: &ClientInfo{
			Hostname:   "test-host-pc",
			Platform:   "linux",
			Architect:  "amd64",
			AppVersion: "v1.2.3",
		},

		Checksum: "abc123def456",
	}
}

func setupTestSummaryJSON(t *testing.T, filename string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, filename)
	testSummary := setupRepoSummary(t)

	if _, err := files.JSONWrite(tmpFile, &testSummary, false); err != nil {
		t.Fatalf("failed to write test JSON file: %v", err)
	}

	return tmpFile
}

func setupRepoProcessor(t *testing.T) *RepoProcessor {
	t.Helper()

	c := ui.NewConsole(
		ui.WithFrame(frame.New(frame.WithColorBorder(color.BrightGray))),
		ui.WithTerminal(terminal.New(
			terminal.WithReader(strings.NewReader("y\n")),
			terminal.WithWriter(io.Discard), // send output to null, show no prompt
		)),
	)
	g, _ := NewManager("")
	a := &config.Config{
		DBName: "test.db",
		DBPath: "/tmp/testpath",
		Path:   &config.Path{},
		Info:   &config.Information{Version: "1.2.3"},
	}

	return NewRepoProcessor(c, g, a)
}

func TestReadSummary(t *testing.T) {
	t.Parallel()
	rp := setupRepoProcessor(t)

	summaryFile := setupTestSummaryJSON(t, SummaryFileName)
	g, err := rp.readSummary(filepath.Dir(summaryFile))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	g.GenChecksum()
	w := setupRepoSummary(t)

	tests := []struct {
		name string
		want any
		got  any
	}{
		// git metadata
		{"GitBranch", w.GitBranch, g.GitBranch},
		{"GitRemote", w.GitRemote, g.GitRemote},
		{"LastSync", w.LastSync, g.LastSync},
		{"ConflictResolution", w.ConflictResolution, g.ConflictResolution},
		{"HashAlgorithm", w.HashAlgorithm, g.HashAlgorithm},
		// repo stats
		{"Bookmarks", w.RepoStats.Bookmarks, g.RepoStats.Bookmarks},
		{"Name", w.RepoStats.Name, g.RepoStats.Name},
		{"Favorites", w.RepoStats.Favorites, g.RepoStats.Favorites},
		{"Tags", w.RepoStats.Tags, g.RepoStats.Tags},
		// client info
		{"Hostname", w.ClientInfo.Hostname, g.ClientInfo.Hostname},
		{"Platform", w.ClientInfo.Platform, g.ClientInfo.Platform},
		{"Architect", w.ClientInfo.Architect, g.ClientInfo.Architect},
		{"AppVersion", w.ClientInfo.AppVersion, g.ClientInfo.AppVersion},
	}

	for _, tt := range tests {
		if tt.want != tt.got {
			t.Errorf("%s: want %v, got %v", tt.name, tt.want, tt.got)
		}
	}

	if w.Checksum == g.Checksum {
		t.Fatalf("checksum must be different: %s == %s", w.Checksum, g.Checksum)
	}
}
