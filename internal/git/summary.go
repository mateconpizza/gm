package git

import (
	"strconv"
	"strings"

	"github.com/mateconpizza/gm/internal/format"
)

type RepoStats struct {
	Name      string `json:"dbname"`
	Bookmarks int    `json:"bookmarks"`
	Tags      int    `json:"tags"`
	Favorites int    `json:"favorites"`
}

type ClientInfo struct {
	Hostname   string `json:"hostname"`
	Platform   string `json:"platform"`
	Architect  string `json:"architecture"`
	AppVersion string `json:"app_version"`
}

type SyncGitSummary struct {
	GitBranch          string      `json:"git_branch"`
	GitRemote          string      `json:"git_remote"`
	LastSync           string      `json:"last_sync"`
	ConflictResolution string      `json:"conflict_resolution"`
	HashAlgorithm      string      `json:"hash_algorithm"`
	RepoStats          *RepoStats  `json:"stats"`
	ClientInfo         *ClientInfo `json:"client_info"`
	Checksum           string      `json:"checksum"`
}

// GenerateChecksum generates a checksum for the SyncGitSummary.
func (s *SyncGitSummary) GenerateChecksum() {
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

	s.Checksum = format.GenerateHash(sb.String(), length)
}

func NewSummary() *SyncGitSummary {
	return &SyncGitSummary{}
}
