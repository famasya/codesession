package main

import "fmt"

func CleanupWorktree(threadID string) error {
	// Prefer repoPath from session; otherwise infer from known worktree layout
	sessionData := lazyLoadSession(threadID)
	var worktreePath, repoPath string
	if sessionData != nil && sessionData.RepositoryPath != "" {
		repoPath = sessionData.RepositoryPath
		worktreePath = sessionData.WorktreePath
	} else {
		return fmt.Errorf("cannot determine repository path for cleanup without session data for thread %s", threadID)
	}
	return gitOps.RemoveWorktree(repoPath, worktreePath)
}
