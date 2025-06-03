package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mateconpizza/gm/internal/bookmark"
	"github.com/mateconpizza/gm/internal/config"
	"github.com/mateconpizza/gm/internal/git"
	"github.com/mateconpizza/gm/internal/handler"
	"github.com/mateconpizza/gm/internal/repo"
	"github.com/mateconpizza/gm/internal/slice"
	"github.com/mateconpizza/gm/internal/sys/files"
)

// diffDeletedBookmarks checks for deleted bookmarks.
func diffDeletedBookmarks(r *repo.SQLiteRepository) error {
	dbName := strings.TrimSuffix(r.Cfg.Name, filepath.Ext(r.Cfg.Name))
	root := filepath.Join(config.App.Path.Git, "data", dbName)
	jsonBookmarks := slice.New[bookmark.Bookmark]()
	if err := bookmark.LoadJSONBookmarks(root, jsonBookmarks); err != nil {
		return fmt.Errorf("loading JSON bookmarks: %w", err)
	}
	bb, err := r.All()
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	bookmarks := slice.New[bookmark.Bookmark]()
	bookmarks.Set(&bb)
	diff := bookmark.FindChanged(bookmarks.ItemsPtr(), jsonBookmarks.ItemsPtr())
	if len(diff) == 0 {
		return nil
	}

	for _, b := range diff {
		if _, ok := r.Has(b.URL); ok {
			continue
		}
		if err := bookmark.CleanupFiles(root, b.URL); err != nil {
			return fmt.Errorf("cleanup files: %w", err)
		}
	}
	return nil
}

type gitFlagsType struct {
	origin string
}

var gitFlags = gitFlagsType{}

var gitPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Fetch from and integrate with another repository or a local branch.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.Fetch(config.App.Path.Git)
	},
}

var gitPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Update remote refs along with associated objects.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.PushChanges(config.App.Path.Git)
	},
}

var gitCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Record changes to the repository.",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := config.App.Path.Git
		r, err := repo.New(config.App.DBPath)
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		defer r.Close()
		bb, err := r.All()
		if err != nil {
			return fmt.Errorf("%w", err)
		}
		bs := slice.New[bookmark.Bookmark]()
		bs.Set(&bb)
		if bs.Empty() {
			return repo.ErrRecordNotFound
		}
		if err := bookmark.ExportBookmarks(repoPath, config.App.DBName, bs.ItemsPtr()); err != nil {
			return fmt.Errorf("creating structure: %w", err)
		}
		if err := diffDeletedBookmarks(r); err != nil {
			return fmt.Errorf("checking for deleted records: %w", err)
		}
		if hasChanges, _ := git.HasChanges(repoPath); !hasChanges {
			return git.ErrGitNothingToCommit
		}
		sum, err := git.UpdatedSummary(repoPath, config.App.Info.Version)
		if err != nil {
			return fmt.Errorf("updating summary: %w", err)
		}
		jsonFile := filepath.Join(repoPath, "summary.json")
		if err := files.JSONWrite(jsonFile, sum, true); err != nil {
			return fmt.Errorf("saving summary: %w", err)
		}
		if err := git.AddAll(repoPath); err != nil {
			return fmt.Errorf("git add: %w", err)
		}
		status, err := git.Status(repoPath)
		if err != nil {
			return fmt.Errorf("git status: %w", err)
		}
		return git.CommitChanges(repoPath, status)
	},
}

var gitRemoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage set of tracked repositories.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.AddRemote(config.App.Path.Git, gitFlags.origin)
	},
}

var gitLogCmd = &cobra.Command{
	Use:   "log",
	Short: "Show commit logs.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return git.Log(config.App.Path.Git)
	},
}

var gitInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create empty Git repository.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if files.Exists(config.App.Path.Git) {
			return nil
		}
		return files.MkdirAll(config.App.Path.Git)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		repoPath := config.App.Path.Git
		if err := git.InitRepo(repoPath, config.App.Force); err != nil {
			return fmt.Errorf("%w", err)
		}
		jfile := filepath.Join(repoPath, "summary.json")
		if err := files.JSONWrite(jfile, git.NewSummary(repoPath, config.App.Info.Version), true); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := git.AddAll(repoPath); err != nil {
			return fmt.Errorf("%w", err)
		}
		if err := git.CommitChanges(repoPath, "initial commit"); err != nil {
			return fmt.Errorf("%w", err)
		}
		return nil
	},
}

// gitCmd represents the git command.
var gitCmd = &cobra.Command{
	Use:     "git",
	Short:   "Git commands",
	Aliases: []string{"g"},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := handler.AssertDefaultDatabaseExists(); err != nil {
			return fmt.Errorf("%w", err)
		}
		switch cmd.Name() {
		case "init", "git":
			return nil
		}
		if !git.IsInitialized(config.App.Path.Git) {
			return git.ErrGitNotInitialized
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
}

var gitTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Test git commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	gitRemoteCmd.Flags().StringVar(&gitFlags.origin, "add", "", "git remote origin")
	_ = gitRemoteCmd.MarkFlagRequired("add")
	gitCmd.AddCommand(gitPullCmd, gitPushCmd, gitCommitCmd, gitRemoteCmd, gitInitCmd, gitLogCmd, gitTestCmd)
	rootCmd.AddCommand(gitCmd)
}
