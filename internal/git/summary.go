package git

import (
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/dbtask"
	"github.com/mateconpizza/gm/internal/ui/txt"
)

const SummaryFileName = "summary.json"

// ClientInfo holds information about the client machine and application.
type ClientInfo struct {
	Hostname   string `json:"hostname"`     // Hostname is the client's hostname.
	Platform   string `json:"platform"`     // Platform is the client's operating system platform.
	Architect  string `json:"architecture"` // Architect is the client's system architecture.
	AppVersion string `json:"app_version"`  // AppVersion is the application's version.
}

// SyncGitSummary summarizes the state and metadata of a Git-synced repository.
type SyncGitSummary struct {
	GitBranch          string            `json:"git_branch"`          // GitBranch is the current Git branch.
	GitRemote          string            `json:"git_remote"`          // GitRemote is the Git remote URL.
	LastSync           string            `json:"last_sync"`           // LastSync is the timestamp of the last sync.
	ConflictResolution string            `json:"conflict_resolution"` // Describes the strategy for resolving conflicts.
	HashAlgorithm      string            `json:"hash_algorithm"`      // Specifies the algorithm used for checksums.
	RepoStats          *dbtask.RepoStats `json:"stats"`               // RepoStats contains statistics for the repository.
	ClientInfo         *ClientInfo       `json:"client_info"`         // ClientInfo contains details about the client.
	Checksum           string            `json:"checksum"`            // Checksum is the summary's generated checksum.
}

// GenChecksum generates a checksum for the SyncGitSummary.
func (s *SyncGitSummary) GenChecksum() {
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

	s.Checksum = txt.GenHash(sb.String(), length)
}

func NewSummary() *SyncGitSummary {
	return &SyncGitSummary{}
}
