package gitops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mateconpizza/gm/internal/locker/enc"
	"github.com/mateconpizza/gm/internal/locker/gpg"
)

var ErrRepoConfig = errors.New("gitops: invalid repo config")

const (
	RepoConfigFile    = ".repo.json"
	RepoConfigVersion = 1
)

// RepoFormat is how bookmarks are serialized in the repo.
type RepoFormat string

const (
	RepoFormatJSON RepoFormat = "json"
	RepoFormatYAML RepoFormat = "yaml"
)

// RepoConfig describes how a git repo stores its bookmarks. It lives in
// RepoConfigFile at the repo root, so a clone knows how to read itself.
type RepoConfig struct {
	Version    int            `json:"version"`
	Format     RepoFormat     `json:"format"`
	Encryption enc.Encryption `json:"encryption"`
}

// DefaultRepoConfig is the plain JSON setup.
func DefaultRepoConfig() RepoConfig {
	return RepoConfig{
		Version:    RepoConfigVersion,
		Format:     RepoFormatJSON,
		Encryption: enc.None,
	}
}

func (c RepoConfig) Validate() error {
	switch {
	case c.Version < 1:
		return fmt.Errorf("%w: version %d", ErrRepoConfig, c.Version)
	case c.Version > RepoConfigVersion:
		return fmt.Errorf("%w: version %d is newer than this gm supports (%d), upgrade gm",
			ErrRepoConfig, c.Version, RepoConfigVersion)
	}

	switch c.Format {
	case RepoFormatJSON, RepoFormatYAML:
	default:
		return fmt.Errorf("%w: unknown format %q", ErrRepoConfig, c.Format)
	}

	switch c.Encryption {
	case enc.None, enc.GPG, enc.AESGCM:
	default:
		return fmt.Errorf("%w: unknown encryption %q", ErrRepoConfig, c.Encryption)
	}

	return nil
}

// LoadRepoConfig reads the config from the repo root. A missing file returns
// an error that matches os.ErrNotExist, so callers can fall back to legacy repos.
func LoadRepoConfig(root string) (RepoConfig, error) {
	data, err := os.ReadFile(filepath.Join(root, RepoConfigFile))
	if err != nil {
		return RepoConfig{}, err
	}

	var c RepoConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return RepoConfig{}, fmt.Errorf("%w: %w", ErrRepoConfig, err)
	}
	if err := c.Validate(); err != nil {
		return RepoConfig{}, err
	}

	return c, nil
}

// Save validates and writes the config to the repo root.
func (c RepoConfig) Save(root string) error {
	if err := c.Validate(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(root, RepoConfigFile), append(data, '\n'), 0o600)
}

func ResolveConfig(root string) (RepoConfig, error) {
	cfg, err := LoadRepoConfig(root)
	if err == nil {
		return cfg, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return RepoConfig{}, err // a corrupt file is an error, not "legacy"
	}

	// legacy repo: no config file, use the old markers
	cfg = DefaultRepoConfig()
	if gpg.IsInitialized(root) {
		cfg.Encryption = enc.GPG
	}

	return cfg, nil
}
