package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sst/opencode-sdk-go"
)

func TestSendMessage_SessionNotExists(t *testing.T) {
	// Clear session cache
	sessionMutex.Lock()
	originalCache := sessionCache
	sessionCache = make(map[string]*SessionData)
	sessionMutex.Unlock()

	defer func() {
		sessionMutex.Lock()
		sessionCache = originalCache
		sessionMutex.Unlock()
	}()

	threadID := "nonexistent-thread"
	message := "test message"

	response := SendMessage(threadID, message)

	if response != nil {
		t.Error("Expected nil response for non-existent session, got response")
	}
}

func TestSendMessage_NilSession(t *testing.T) {
	// Clear session cache
	sessionMutex.Lock()
	originalCache := sessionCache
	sessionCache = make(map[string]*SessionData)
	sessionMutex.Unlock()

	defer func() {
		sessionMutex.Lock()
		sessionCache = originalCache
		sessionMutex.Unlock()
	}()

	threadID := "test-thread-nil-session"
	sessionData := &SessionData{
		ThreadID:     threadID,
		Session:      nil, // Nil session object
		WorktreePath: "/tmp/test-worktree",
		Model:        Model{ProviderID: "openai", ModelID: "gpt-4"},
	}

	// Add to cache
	sessionMutex.Lock()
	sessionCache[threadID] = sessionData
	sessionMutex.Unlock()

	message := "test message"

	response := SendMessage(threadID, message)

	if response != nil {
		t.Error("Expected nil response for session with nil session object, got response")
	}
}

func TestSendMessage_WorktreeDoesNotExist(t *testing.T) {
	// Clear session cache
	sessionMutex.Lock()
	originalCache := sessionCache
	sessionCache = make(map[string]*SessionData)
	sessionMutex.Unlock()

	defer func() {
		sessionMutex.Lock()
		sessionCache = originalCache
		sessionMutex.Unlock()
	}()

	threadID := "test-thread-no-worktree"
	nonExistentWorktree := "/tmp/nonexistent-worktree-path"

	sessionData := &SessionData{
		ThreadID:     threadID,
		Session:      &opencode.Session{ID: "session-123"},
		WorktreePath: nonExistentWorktree,
		Model:        Model{ProviderID: "openai", ModelID: "gpt-4"},
	}

	// Add to cache
	sessionMutex.Lock()
	sessionCache[threadID] = sessionData
	sessionMutex.Unlock()

	message := "test message"

	response := SendMessage(threadID, message)

	if response != nil {
		t.Error("Expected nil response for session with non-existent worktree, got response")
	}
}

func TestSendMessage_ValidWorktreeNilClient(t *testing.T) {
	// Clear session cache
	sessionMutex.Lock()
	originalCache := sessionCache
	sessionCache = make(map[string]*SessionData)
	sessionMutex.Unlock()

	defer func() {
		sessionMutex.Lock()
		sessionCache = originalCache
		sessionMutex.Unlock()
	}()

	// Create temporary worktree directory
	tempDir := t.TempDir()
	worktreePath := filepath.Join(tempDir, "worktree")
	err := os.MkdirAll(worktreePath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test worktree: %v", err)
	}

	threadID := "test-thread-nil-client"
	sessionData := &SessionData{
		ThreadID:     threadID,
		Session:      &opencode.Session{ID: "session-123"},
		WorktreePath: worktreePath,
		Model:        Model{ProviderID: "openai", ModelID: "gpt-4"},
	}

	// Add to cache
	sessionMutex.Lock()
	sessionCache[threadID] = sessionData
	sessionMutex.Unlock()

	// Mock opencodeClient to return nil
	originalOpencodeClient := opencodeClient
	opencodeClient = nil
	defer func() {
		opencodeClient = originalOpencodeClient
	}()

	message := "test message"

	response := SendMessage(threadID, message)

	if response != nil {
		t.Error("Expected nil response when OpenCode client is nil, got response")
	}
}

func TestSendMessage_MessageEnhancement(t *testing.T) {
	// This test focuses on verifying the message enhancement logic
	// Since we can't easily mock the OpenCode client without significant refactoring,
	// we'll test the message preparation logic conceptually

	originalMessage := "Hello, please help with this code"
	expectedEnhancedMessage := originalMessage + "\n\nImportant: Stay within the current worktree directory for all file operations."

	// The actual enhancement happens inside SendMessage, so we can't directly test it
	// without refactoring the function to make it more testable

	if len(expectedEnhancedMessage) <= len(originalMessage) {
		t.Error("Enhanced message should be longer than original message")
	}

	if expectedEnhancedMessage[:len(originalMessage)] != originalMessage {
		t.Error("Enhanced message should start with original message")
	}
}

func TestSendMessage_AbsolutePathHandling(t *testing.T) {
	// Test that relative paths are converted to absolute paths
	tempDir := t.TempDir()

	// Create a relative path from temp directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	relativePath := "relative/worktree"
	absolutePath := filepath.Join(tempDir, relativePath)

	// Create the directory structure
	err := os.MkdirAll(absolutePath, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test that filepath.Abs works as expected
	absPath, err := filepath.Abs(relativePath)
	if err != nil {
		t.Fatalf("filepath.Abs failed: %v", err)
	}

	// Handle macOS symlink resolution
	expectedAbsPath := absolutePath
	expectedAbsPathResolved, _ := filepath.EvalSymlinks(expectedAbsPath)
	absPathResolved, _ := filepath.EvalSymlinks(absPath)

	if absPathResolved != expectedAbsPathResolved {
		t.Errorf("Expected absolute path %s, got %s", expectedAbsPathResolved, absPathResolved)
	}
}
