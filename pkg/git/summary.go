package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSummaryMissingName   = errors.New("summary: missing repo name")
	ErrSummaryChecksumEmpty = errors.New("summary: checksum empty")
)

const SummaryFileName = "summary.json" // summary.json

type StatsLoader func(ctx context.Context, dest any) error

// ClientInfo holds information about the client machine and application.
type ClientInfo struct {
	Hostname   string `json:"hostname"`     // Hostname is the client's hostname.
	Platform   string `json:"platform"`     // Platform is the client's operating system platform.
	Architect  string `json:"architecture"` // Architect is the client's system architecture.
	AppVersion string `json:"app_version"`  // AppVersion is the application's version.
}

// Summary summarizes the state and metadata of a Git-synced repository.
type Summary struct {
	GitBranch          string      `json:"git_branch"`          // GitBranch is the current Git branch.
	GitRemote          string      `json:"git_remote"`          // GitRemote is the Git remote URL.
	LastSync           string      `json:"last_sync"`           // LastSync is the timestamp of the last sync.
	ConflictResolution string      `json:"conflict_resolution"` // Describes the strategy for resolving conflicts.
	HashAlgorithm      string      `json:"hash_algorithm"`      // Specifies the algorithm used for checksums.
	RepoStats          *RepoStats  `json:"stats"`               // RepoStats contains statistics for the repository.
	ClientInfo         *ClientInfo `json:"client_info"`         // ClientInfo contains details about the client.
	Checksum           string      `json:"checksum"`            // Checksum is the summary's generated checksum.
}

// GenChecksum generates a checksum for the SyncGitSummary.
func (s *Summary) GenChecksum() {
	const length = 12

	var sb strings.Builder
	sb.WriteString(s.GitBranch)
	sb.WriteString(s.GitRemote)
	sb.WriteString(s.ConflictResolution)
	sb.WriteString(s.HashAlgorithm)

	if s.RepoStats != nil {
		sb.WriteString(s.RepoStats.Name)
		sb.WriteString(strconv.Itoa(s.RepoStats.Bookmarks))
		sb.WriteString(strconv.Itoa(s.RepoStats.Tags))
		sb.WriteString(strconv.Itoa(s.RepoStats.Favorites))
	}

	if s.ClientInfo != nil {
		sb.WriteString(s.ClientInfo.Hostname)
		sb.WriteString(s.ClientInfo.Platform)
		sb.WriteString(s.ClientInfo.Architect)
		sb.WriteString(s.ClientInfo.AppVersion)
	}

	s.Checksum = genHash(sb.String(), length)
}

func (s *Summary) Validate() error {
	if err := s.RepoStats.Validate(); err != nil {
		return err
	}

	if s.Checksum == "" {
		return ErrSummaryChecksumEmpty
	}

	s.GenChecksum()

	return nil
}

// RepoStats holds metadata about a bookmark repository.
type RepoStats struct {
	Name        string `db:"-"               json:"name"`
	Bookmarks   int    `db:"total_bookmarks" json:"bookmarks"`
	Tags        int    `db:"total_tags"      json:"tags"`
	Favorites   int    `db:"favorites"       json:"favorites"`
	Archived    int    `db:"archived"        json:"archived"`
	DeadLinks   int    `db:"dead_links"      json:"dead_links"`
	TotalVisits int    `db:"total_visits"    json:"total_visits"`
}

func (rs *RepoStats) Validate() error {
	if rs.Name == "" {
		return ErrSummaryMissingName
	}

	return nil
}

func (rs *RepoStats) String() string {
	var parts []string
	if rs.Bookmarks > 0 {
		parts = append(parts, fmt.Sprintf("%d bookmarks", rs.Bookmarks))
	}

	if rs.Tags > 0 {
		parts = append(parts, fmt.Sprintf("%d tags", rs.Tags))
	}

	if rs.Favorites > 0 {
		parts = append(parts, fmt.Sprintf("%d favorites", rs.Favorites))
	}

	if rs.TotalVisits > 0 {
		parts = append(parts, fmt.Sprintf("%d visits", rs.TotalVisits))
	}

	if len(parts) == 0 {
		parts = append(parts, "no bookmarks")
	}

	return strings.Join(parts, ", ")
}

func summaryComplete(ctx context.Context, g *Git, s *RepoStats, ver string) (*Summary, error) {
	branch, err := g.Branch(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting branch: %w", err)
	}

	remote, err := g.Remote(ctx)
	if err != nil {
		remote = ""
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("getting hostname: %w", err)
	}

	summary := &Summary{
		GitBranch:          branch,
		GitRemote:          remote,
		LastSync:           time.Now().Format(time.RFC3339),
		ConflictResolution: "timestamp",
		HashAlgorithm:      "SHA-256",
		ClientInfo: &ClientInfo{
			Hostname:   hostname,
			Platform:   runtime.GOOS,
			Architect:  runtime.GOARCH,
			AppVersion: ver,
		},
		RepoStats: s,
	}

	summary.GenChecksum()

	return summary, nil
}

func NewSummary() *Summary {
	return &Summary{}
}

func NewRepoStats() *RepoStats {
	return &RepoStats{}
}
