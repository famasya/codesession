package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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

// CreateWorktree creates a new git worktree at the specified path
func (g *GitOperations) CreateWorktree(repoPath, worktreePath string) error {
	slog.Debug("creating worktree", "repo_path", repoPath, "worktree_path", worktreePath)

	// Create the worktree directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0755); err != nil {
		return fmt.Errorf("failed to create worktree parent directory: %w", err)
	}

	// Clone the repository to the worktree path
	// This is similar to creating a linked worktree but uses go-git's PlainClone
	_, err := git.PlainClone(worktreePath, false, &git.CloneOptions{
		URL: repoPath,
	})
	if err != nil {
		// If it's a file system path, we need to handle it differently
		// Try to initialize and copy files manually
		return g.createWorktreeManually(repoPath, worktreePath)
	}

	slog.Debug("worktree created successfully", "worktree_path", worktreePath)
	return nil
}

// createWorktreeManually creates a worktree by copying the git repository
func (g *GitOperations) createWorktreeManually(repoPath, worktreePath string) error {
	// Open the source repository
	sourceRepo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("failed to open source repository at %s: %w", repoPath, err)
	}

	// Get the current HEAD commit
	head, err := sourceRepo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Get remote configuration from source repository
	sourceRemote, err := sourceRepo.Remote("origin")
	var remoteURL string
	if err == nil && len(sourceRemote.Config().URLs) > 0 {
		remoteURL = sourceRemote.Config().URLs[0]
	}

	// Clone the repository using the remote URL if available, otherwise use local path
	var targetRepo *git.Repository
	if remoteURL != "" {
		// Clone from remote URL
		targetRepo, err = git.PlainClone(worktreePath, false, &git.CloneOptions{
			URL: remoteURL,
		})
		if err != nil {
			slog.Warn("failed to clone from remote, falling back to local clone", "error", err)
			// Fallback to local clone
			targetRepo, err = g.cloneFromLocal(sourceRepo, worktreePath, head)
		}
	} else {
		// Clone from local repository
		targetRepo, err = g.cloneFromLocal(sourceRepo, worktreePath, head)
	}

	if err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Checkout to the same commit as HEAD
	targetWorktree, err := targetRepo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get target worktree: %w", err)
	}

	err = targetWorktree.Checkout(&git.CheckoutOptions{
		Hash: head.Hash(),
	})
	if err != nil {
		slog.Warn("failed to checkout specific commit, using default branch", "error", err)
	}

	return nil
}

// cloneFromLocal creates a clone from a local repository
func (g *GitOperations) cloneFromLocal(sourceRepo *git.Repository, worktreePath string, head *plumbing.Reference) (*git.Repository, error) {
	// Initialize a new repository at the worktree path
	targetRepo, err := git.PlainInit(worktreePath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize repository at %s: %w", worktreePath, err)
	}

	// Get all references from source repository
	refs, err := sourceRepo.References()
	if err != nil {
		return nil, fmt.Errorf("failed to get references from source: %w", err)
	}

	// Copy all objects from source to target
	sourceStorer := sourceRepo.Storer
	targetStorer := targetRepo.Storer

	// Iterate through all references and copy them
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		// Copy the reference to target repository
		return targetStorer.SetReference(ref)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy references: %w", err)
	}

	// Copy all objects (commits, trees, blobs)
	objectIter, err := sourceStorer.IterEncodedObjects(plumbing.AnyObject)
	if err != nil {
		return nil, fmt.Errorf("failed to get objects from source: %w", err)
	}

	err = objectIter.ForEach(func(obj plumbing.EncodedObject) error {
		_, err := targetStorer.SetEncodedObject(obj)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy objects: %w", err)
	}

	return targetRepo, nil
}

// copyGitConfig copies git configuration from source to target
func (g *GitOperations) copyGitConfig(sourcePath, targetPath string) error {
	sourceConfigPath := filepath.Join(sourcePath, ".git", "config")
	targetConfigPath := filepath.Join(targetPath, ".git", "config")

	sourceData, err := os.ReadFile(sourceConfigPath)
	if err != nil {
		return err
	}

	return os.WriteFile(targetConfigPath, sourceData, 0644)
}

// RemoveWorktree removes a git worktree at the specified path
func (g *GitOperations) RemoveWorktree(worktreePath string) error {
	slog.Debug("removing worktree", "worktree_path", worktreePath)

	// Check if the worktree directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		slog.Debug("worktree directory does not exist, nothing to remove", "worktree_path", worktreePath)
		return nil
	}

	// Remove the entire worktree directory
	err := os.RemoveAll(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to remove worktree directory %s: %w", worktreePath, err)
	}

	slog.Debug("worktree removed successfully", "worktree_path", worktreePath)
	return nil
}

// GetStatus gets the status of a git repository at the specified path
func (g *GitOperations) GetStatus(worktreePath string) (*GitStatus, error) {
	slog.Debug("getting git status", "worktree_path", worktreePath)

	// Open the repository at the worktree path
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get status
	status, err := worktree.Status()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// Parse status into our structure
	gitStatus := &GitStatus{
		IsClean:        status.IsClean(),
		ModifiedFiles:  make([]string, 0),
		UntrackedFiles: make([]string, 0),
		StagedFiles:    make([]string, 0),
	}

	for file, fileStatus := range status {
		if fileStatus.Staging != git.Unmodified {
			gitStatus.StagedFiles = append(gitStatus.StagedFiles, file)
		}
		if fileStatus.Worktree == git.Modified || fileStatus.Worktree == git.Renamed {
			gitStatus.ModifiedFiles = append(gitStatus.ModifiedFiles, file)
		}
		if fileStatus.Worktree == git.Untracked {
			gitStatus.UntrackedFiles = append(gitStatus.UntrackedFiles, file)
		}
	}

	slog.Debug("git status retrieved", "worktree_path", worktreePath, "is_clean", gitStatus.IsClean,
		"modified_count", len(gitStatus.ModifiedFiles), "untracked_count", len(gitStatus.UntrackedFiles),
		"staged_count", len(gitStatus.StagedFiles))

	return gitStatus, nil
}

// AddAll stages all changes in the repository
func (g *GitOperations) AddAll(worktreePath string) error {
	slog.Debug("staging all changes", "worktree_path", worktreePath)

	// Open the repository
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Add all changes (equivalent to 'git add .')
	_, err = worktree.Add(".")
	if err != nil {
		return fmt.Errorf("failed to add changes: %w", err)
	}

	slog.Debug("all changes staged successfully", "worktree_path", worktreePath)
	return nil
}

// Commit creates a commit with the specified message and returns the commit hash
func (g *GitOperations) Commit(worktreePath, message string) (string, error) {
	slog.Debug("creating commit", "worktree_path", worktreePath, "message", message)

	// Open the repository
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get the worktree
	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create commit
	commit, err := worktree.Commit(message, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "CodeSession Bot",
			Email: "codesession-bot@example.com",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	commitHash := commit.String()
	slog.Debug("commit created successfully", "worktree_path", worktreePath, "commit_hash", commitHash)
	return commitHash, nil
}

// GetCurrentBranch returns the current branch name
func (g *GitOperations) GetCurrentBranch(worktreePath string) (string, error) {
	slog.Debug("getting current branch", "worktree_path", worktreePath)

	// Open the repository
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get HEAD reference
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Extract branch name from reference
	branchName := ""
	if head.Name().IsBranch() {
		branchName = head.Name().Short()
	} else {
		// If HEAD is detached, return the hash
		branchName = head.Hash().String()[:8]
	}

	slog.Debug("current branch retrieved", "worktree_path", worktreePath, "branch", branchName)
	return branchName, nil
}

// Push pushes the specified branch to the remote origin
func (g *GitOperations) Push(worktreePath, branch string) error {
	slog.Debug("pushing to remote", "worktree_path", worktreePath, "branch", branch)

	// Open the repository
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get remote
	remote, err := repo.Remote("origin")
	if err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	// Push to remote
	refSpec := config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", branch, branch))
	err = remote.Push(&git.PushOptions{
		RefSpecs: []config.RefSpec{refSpec},
	})

	// Handle common push errors
	if err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			slog.Debug("repository already up to date", "worktree_path", worktreePath, "branch", branch)
			return nil
		}
		return fmt.Errorf("failed to push to remote: %w", err)
	}

	slog.Debug("pushed to remote successfully", "worktree_path", worktreePath, "branch", branch)
	return nil
}

// GetCommitHash returns the hash of the current HEAD commit
func (g *GitOperations) GetCommitHash(worktreePath string) (string, error) {
	slog.Debug("getting commit hash", "worktree_path", worktreePath)

	// Open the repository
	repo, err := git.PlainOpen(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository at %s: %w", worktreePath, err)
	}

	// Get HEAD reference
	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	commitHash := head.Hash().String()
	slog.Debug("commit hash retrieved", "worktree_path", worktreePath, "commit_hash", commitHash)
	return commitHash, nil
}

// GetDiff returns the diff of changes in the repository
func (g *GitOperations) GetDiff(worktreePath string) (string, error) {
	slog.Debug("getting git diff", "worktree_path", worktreePath)

	// Simply execute git diff in the worktree directory
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
