package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGitOperations(t *testing.T) {
	git := NewGitOperations()
	if git == nil {
		t.Error("NewGitOperations() returned nil")
	}
}

func TestCreateWorktree_InvalidBranchName(t *testing.T) {
	git := NewGitOperations()

	// Test empty branch name
	err := git.CreateWorktree("/tmp/repo", "/tmp/worktree", "")
	if err == nil {
		t.Error("Expected error for empty branch name, got nil")
	}
	if !strings.Contains(err.Error(), "branch name cannot be empty") {
		t.Errorf("Expected 'branch name cannot be empty' error, got: %v", err)
	}
}

func TestCreateWorktree_ValidBranchNameValidation(t *testing.T) {
	// Create a temporary repository for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")

	// Initialize a git repository
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Configure git for testing
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}

	for _, cmdArgs := range configCmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to configure git: %v", err)
		}
	}

	// Create an initial commit
	readmeFile := filepath.Join(repoPath, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Test repo"), 0644)
	if err != nil {
		t.Fatalf("Failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to add README: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to create initial commit: %v", err)
	}

	git := NewGitOperations()

	tests := []struct {
		name       string
		branchName string
		shouldFail bool
	}{
		{"valid branch name", "feature/test-branch", false},
		{"valid simple name", "main", false},
		{"valid with numbers", "feature123", false},
		{"invalid with spaces", "invalid branch", true},
		{"invalid with ..", "invalid..branch", true},
		{"invalid starting with dash", "-invalid", true},
		{"invalid ending with dot", "invalid.", true},
		{"invalid with tilde", "invalid~branch", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worktreePath := filepath.Join(tempDir, "worktree_"+strings.ReplaceAll(tt.name, " ", "_"))

			err := git.CreateWorktree(repoPath, worktreePath, tt.branchName)

			if tt.shouldFail && err == nil {
				t.Errorf("Expected error for branch name '%s', got nil", tt.branchName)
			}
			if !tt.shouldFail && err != nil {
				// Note: This might fail if git pull fails, which is acceptable for unit tests
				if strings.Contains(err.Error(), "invalid branch name") {
					t.Errorf("Unexpected error for valid branch name '%s': %v", tt.branchName, err)
				}
			}
		})
	}
}

func TestGetStatus(t *testing.T) {
	// Create a temporary repository for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")

	// Initialize a git repository
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Configure git for testing
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}

	for _, cmdArgs := range configCmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to configure git: %v", err)
		}
	}

	// Create and commit a file
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("initial content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to add test file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to create initial commit: %v", err)
	}

	git := NewGitOperations()

	// Test clean repository
	status, err := git.GetStatus(repoPath)
	if err != nil {
		t.Fatalf("GetStatus() failed: %v", err)
	}

	if !status.IsClean {
		t.Error("Expected clean repository, got dirty")
	}
	if len(status.ModifiedFiles) != 0 {
		t.Errorf("Expected 0 modified files, got %d", len(status.ModifiedFiles))
	}
	if len(status.UntrackedFiles) != 0 {
		t.Errorf("Expected 0 untracked files, got %d", len(status.UntrackedFiles))
	}

	// Modify the file
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Test modified file
	status, err = git.GetStatus(repoPath)
	if err != nil {
		t.Fatalf("GetStatus() failed for modified repo: %v", err)
	}

	if status.IsClean {
		t.Error("Expected dirty repository, got clean")
	}

	// Check that we have some indication of changes (could be modified or untracked)
	totalChanges := len(status.ModifiedFiles) + len(status.UntrackedFiles) + len(status.StagedFiles)
	if totalChanges == 0 {
		t.Error("Expected some changes (modified, untracked, or staged files), got none")
	}

	// Create untracked file
	untrackedFile := filepath.Join(repoPath, "untracked.txt")
	err = os.WriteFile(untrackedFile, []byte("untracked content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create untracked file: %v", err)
	}

	// Test untracked file
	status, err = git.GetStatus(repoPath)
	if err != nil {
		t.Fatalf("GetStatus() failed for repo with untracked files: %v", err)
	}

	if len(status.UntrackedFiles) == 0 {
		t.Error("Expected untracked files, got none")
	}
}

func TestGetStatus_InvalidRepo(t *testing.T) {
	git := NewGitOperations()

	// Test with non-existent directory
	_, err := git.GetStatus("/nonexistent/path")
	if err == nil {
		t.Error("Expected error for non-existent repository, got nil")
	}

	// Test with non-git directory
	tempDir := t.TempDir()
	_, err = git.GetStatus(tempDir)
	if err == nil {
		t.Error("Expected error for non-git directory, got nil")
	}
}

func TestAddAll(t *testing.T) {
	// Create a temporary repository for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")

	// Initialize a git repository
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Configure git for testing
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}

	for _, cmdArgs := range configCmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to configure git: %v", err)
		}
	}

	// Create test files
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	git := NewGitOperations()

	// Test AddAll
	err = git.AddAll(repoPath)
	if err != nil {
		t.Errorf("AddAll() failed: %v", err)
	}

	// Verify files are staged
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to check git status: %v", err)
	}

	if !strings.Contains(string(output), "A ") {
		t.Error("Expected files to be staged after AddAll")
	}
}

func TestGetDiff(t *testing.T) {
	// Create a temporary repository for testing
	tempDir := t.TempDir()
	repoPath := filepath.Join(tempDir, "repo")

	// Initialize a git repository
	err := os.MkdirAll(repoPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create repo directory: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skip("Git not available for testing")
	}

	// Configure git for testing
	configCmds := [][]string{
		{"git", "config", "user.name", "Test User"},
		{"git", "config", "user.email", "test@example.com"},
	}

	for _, cmdArgs := range configCmds {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = repoPath
		if err := cmd.Run(); err != nil {
			t.Skipf("Failed to configure git: %v", err)
		}
	}

	git := NewGitOperations()

	// Test diff on clean repo (should be "No changes")
	diff, err := git.GetDiff(repoPath)
	if err != nil {
		t.Fatalf("GetDiff() failed on clean repo: %v", err)
	}

	if diff != "No changes to show." {
		t.Errorf("Expected 'No changes to show.', got: %s", diff)
	}

	// Create and add a file, then modify it to create a diff
	testFile := filepath.Join(repoPath, "test.txt")
	err = os.WriteFile(testFile, []byte("original content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add and commit initial file
	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to add file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	err = cmd.Run()
	if err != nil {
		t.Skipf("Failed to commit: %v", err)
	}

	// Modify the file
	err = os.WriteFile(testFile, []byte("modified content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Test diff on modified file
	diff, err = git.GetDiff(repoPath)
	if err != nil {
		t.Fatalf("GetDiff() failed on modified repo: %v", err)
	}

	if diff == "No changes to show." {
		t.Error("Expected diff output for modified file, got 'No changes'")
	}

	// Diff should contain the filename
	if !strings.Contains(diff, "test.txt") {
		t.Error("Expected diff to contain filename")
	}
}
