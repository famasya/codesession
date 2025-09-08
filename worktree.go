package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func CleanupWorktree(threadID string) error {
	// Prefer repoPath from session; otherwise infer from known worktree layout
	sessionData := lazyLoadSession(threadID)
	var worktreePath, repoPath string
	if sessionData != nil && sessionData.RepositoryPath != "" {
		repoPath = sessionData.RepositoryPath
		worktreePath = sessionData.WorktreePath
	} else {
		// try known layout under configured repositories
		for _, repo := range AppConfig.Repositories {
			candidate := filepath.Join(repo.Path, ".worktrees", threadID)
			if _, err := os.Stat(candidate); err == nil {
				repoPath = repo.Path
				worktreePath = candidate
				break
			}
		}
		if repoPath == "" {
			return fmt.Errorf("cannot determine repository path for cleanup without session data for thread %s", threadID)
		}
	}
	slog.Debug("removing worktree", "thread_id", threadID, "repo_path", repoPath, "worktree_path", worktreePath)
	return gitOps.RemoveWorktree(repoPath, worktreePath)
}
