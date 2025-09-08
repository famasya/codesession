package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitStatus represents the status of a Git repository
type GitStatus struct {
	IsClean        bool
	ModifiedFiles  []string
	UntrackedFiles []string
	StagedFiles    []string
}

// GitOperations provides a wrapper around go-git operations
type GitOperations struct{}

// NewGitOperations creates a new GitOperations instance
func NewGitOperations() *GitOperations {
	return &GitOperations{}
}

// CreateWorktree creates a new git worktree at the specified path with a branch
func (g *GitOperations) CreateWorktree(repoPath, worktreePath, branchName string) error {
	slog.Debug("creating worktree", "repo_path", repoPath, "worktree_path", worktreePath, "branch", branchName)

	// Reject empty branch names early
	if branchName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// Validate branch name
	validate := exec.Command("git", "check-ref-format", "--branch", branchName)
	validate.Dir = repoPath
	if out, err := validate.CombinedOutput(); err != nil {
		return fmt.Errorf("invalid branch name %q: %s", branchName, strings.TrimSpace(string(out)))
	}

	// Pull latest changes from remote before creating worktree
	pullCmd := exec.Command("git", "pull", "origin", "main")
	pullCmd.Dir = repoPath
	pullOutput, pullErr := pullCmd.CombinedOutput()
	if pullErr != nil {
		slog.Warn("failed to pull latest changes before creating worktree", "error", pullErr, "output", string(pullOutput))
		// Continue anyway - might be network issues or new repo
	} else {
		slog.Debug("pulled latest changes before creating worktree", "repo_path", repoPath)
	}

	// Create the worktree directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return fmt.Errorf("failed to create worktree parent directory: %w", err)
	}

	// Create git worktree with new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create git worktree: %s", string(output))
	}

	slog.Debug("worktree created successfully", "worktree_path", worktreePath, "branch", branchName)
	return nil
}

// RemoveWorktree removes a git worktree at the specified path
func (g *GitOperations) RemoveWorktree(repoPath, worktreePath string) error {
	slog.Debug("removing worktree", "worktree_path", worktreePath)

	// safety check: avoid to remove main/master
	branch, _ := g.GetCurrentBranch(worktreePath)
	if branch == "main" || branch == "master" || branch == "" {
		return fmt.Errorf("not in a worktree. abort")
	}

	// First try to remove via git worktree remove
	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("git worktree remove failed, falling back to manual removal", "error", err, "output", string(output))

		// Fallback: manually remove directory
		if err := os.RemoveAll(worktreePath); err != nil {
			return fmt.Errorf("failed to remove worktree directory %s: %w", worktreePath, err)
		}
	}

	slog.Debug("worktree removed successfully", "worktree_path", worktreePath)
	return nil
}

// GetStatus gets the status of a git repository at the specified path
func (g *GitOperations) GetStatus(worktreePath string) (*GitStatus, error) {
	slog.Debug("getting git status", "worktree_path", worktreePath)

	cmd := exec.Command("git", "status", "--porcelain=v1 -z")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get git status: %s", string(output))
	}

	// Parse porcelain output
	gitStatus := &GitStatus{
		ModifiedFiles:  make([]string, 0),
		UntrackedFiles: make([]string, 0),
		StagedFiles:    make([]string, 0),
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		stagingStatus := line[0]
		worktreeStatus := line[1]
		filename := line[3:]

		if stagingStatus != ' ' && stagingStatus != '?' {
			gitStatus.StagedFiles = append(gitStatus.StagedFiles, filename)
		}
		if worktreeStatus == 'M' || worktreeStatus == 'D' {
			gitStatus.ModifiedFiles = append(gitStatus.ModifiedFiles, filename)
		}
		if stagingStatus == '?' && worktreeStatus == '?' {
			gitStatus.UntrackedFiles = append(gitStatus.UntrackedFiles, filename)
		}
	}

	gitStatus.IsClean = len(gitStatus.ModifiedFiles) == 0 && len(gitStatus.UntrackedFiles) == 0 && len(gitStatus.StagedFiles) == 0

	slog.Debug("git status retrieved", "worktree_path", worktreePath, "is_clean", gitStatus.IsClean,
		"modified_count", len(gitStatus.ModifiedFiles), "untracked_count", len(gitStatus.UntrackedFiles),
		"staged_count", len(gitStatus.StagedFiles))

	return gitStatus, nil
}

// AddAll stages all changes in the repository
func (g *GitOperations) AddAll(worktreePath string) error {
	slog.Debug("staging all changes", "worktree_path", worktreePath)

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stage changes: %s", string(output))
	}

	slog.Debug("all changes staged successfully", "worktree_path", worktreePath)
	return nil
}

// Commit creates a commit with the specified message and returns the commit hash
func (g *GitOperations) Commit(worktreePath, message string) (string, error) {
	slog.Debug("creating commit", "worktree_path", worktreePath, "message", message)

	cmd := exec.Command("git", "commit", "-m", message, "--author", "codesessions <bot@codesessions.com>", "--no-verify")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", string(output))
	}

	// Get the commit hash
	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashCmd.Dir = worktreePath

	hashOutput, err := hashCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %s", string(hashOutput))
	}

	commitHash := strings.TrimSpace(string(hashOutput))
	slog.Debug("commit created successfully", "worktree_path", worktreePath, "commit_hash", commitHash)
	return commitHash, nil
}

// GetCurrentBranch returns the current branch name
func (g *GitOperations) GetCurrentBranch(worktreePath string) (string, error) {
	slog.Debug("getting current branch", "worktree_path", worktreePath)

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %s", string(output))
	}

	branchName := strings.TrimSpace(string(output))
	slog.Debug("current branch retrieved", "worktree_path", worktreePath, "branch", branchName)
	return branchName, nil
}

// Push pushes the specified branch to the remote origin
func (g *GitOperations) Push(worktreePath, branch string) error {
	slog.Debug("pushing to remote", "worktree_path", worktreePath, "branch", branch)

	// Fetch latest remote state
	fetchCmd := exec.Command("git", "fetch", "origin", branch)
	fetchCmd.Dir = worktreePath
	fetchOutput, fetchErr := fetchCmd.CombinedOutput()
	if fetchErr != nil {
		slog.Warn("failed to fetch before push", "error", fetchErr, "output", string(fetchOutput))
		// Continue with push - might be a new branch
	} else {
		slog.Debug("fetched latest remote state", "worktree_path", worktreePath, "branch", branch)
		
		// Reset to remote state to accept remote as source of truth
		resetCmd := exec.Command("git", "reset", "--hard", "origin/"+branch)
		resetCmd.Dir = worktreePath
		resetOutput, resetErr := resetCmd.CombinedOutput()
		if resetErr != nil {
			slog.Warn("failed to reset to remote state", "error", resetErr, "output", string(resetOutput))
			// Continue with push anyway
		} else {
			slog.Debug("reset to remote state successfully", "worktree_path", worktreePath, "branch", branch)
		}
	}

	cmd := exec.Command("git", "push", "origin", branch)
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if it's just "already up to date"
		if strings.Contains(string(output), "up-to-date") {
			slog.Debug("repository already up to date", "worktree_path", worktreePath, "branch", branch)
			return nil
		}
		return fmt.Errorf("failed to push to remote: %s", string(output))
	}

	slog.Debug("pushed to remote successfully", "worktree_path", worktreePath, "branch", branch)
	return nil
}

// GetCommitHash returns the hash of the current HEAD commit
func (g *GitOperations) GetCommitHash(worktreePath string) (string, error) {
	slog.Debug("getting commit hash", "worktree_path", worktreePath)

	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %s", string(output))
	}

	commitHash := strings.TrimSpace(string(output))
	slog.Debug("commit hash retrieved", "worktree_path", worktreePath, "commit_hash", commitHash)
	return commitHash, nil
}

// GetDiff returns the diff of changes in the repository
func (g *GitOperations) GetDiff(worktreePath string) (string, error) {
	slog.Debug("getting git diff", "worktree_path", worktreePath)

	// Execute git diff in the worktree directory
	cmd := exec.Command("git", "diff", "--minimal", "--ignore-all-space", "--diff-filter=ACMR")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute git diff: %w", err)
	}

	diffOutput := strings.TrimSpace(string(output))

	if diffOutput == "" {
		return "No changes to show.", nil
	}

	// Return the raw diff output - let the sender handle code block formatting
	result := diffOutput

	slog.Debug("git diff executed successfully", "worktree_path", worktreePath, "diff_length", len(result))

	return result, nil
}

// Global GitOperations instance
var gitOps = NewGitOperations()
