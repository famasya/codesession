package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLazyLoadSession_NewSession(t *testing.T) {
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

	threadID := "new-thread-123"

	// Test loading non-existent session
	sessionData := lazyLoadSession(threadID)

	if sessionData != nil {
		t.Error("Expected nil for non-existent session, got data")
	}
}

func TestLazyLoadSession_ExistingInCache(t *testing.T) {
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

	threadID := "cached-thread-456"
	expectedSession := &SessionData{
		ThreadID:  threadID,
		SessionID: "session-789",
		Model:     Model{ProviderID: "openai", ModelID: "gpt-4"},
	}

	// Add to cache
	sessionMutex.Lock()
	sessionCache[threadID] = expectedSession
	sessionMutex.Unlock()

	// Test loading from cache
	sessionData := lazyLoadSession(threadID)

	if sessionData == nil {
		t.Fatal("Expected session data from cache, got nil")
	}

	if sessionData.ThreadID != expectedSession.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", sessionData.ThreadID, expectedSession.ThreadID)
	}

	if sessionData.SessionID != expectedSession.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", sessionData.SessionID, expectedSession.SessionID)
	}
}

func TestLazyLoadSession_LoadFromFile(t *testing.T) {
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

	// Create temporary sessions directory
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, ".sessions")
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create sessions directory: %v", err)
	}

	// Mock sessionsDirectory global variable
	originalSessionsDirectory := sessionsDirectory
	sessionsDirectory = sessionDir
	defer func() {
		sessionsDirectory = originalSessionsDirectory
	}()

	threadID := "file-thread-789"
	expectedSession := SessionData{
		ThreadID:       threadID,
		SessionID:      "session-abc",
		Model:          Model{ProviderID: "openai", ModelID: "gpt-3.5-turbo"},
		WorktreePath:   "/tmp/worktree",
		RepositoryPath: "/repo/path",
		RepositoryName: "test-repo",
		CreatedAt:      time.Now(),
		Commits: []CommitRecord{
			{
				Hash:      "commit123",
				Summary:   "Test commit",
				Timestamp: time.Now(),
				Status:    "success",
			},
		},
	}

	// Write session to file
	sessionFile := filepath.Join(sessionDir, threadID+".json")
	data, err := json.Marshal(expectedSession)
	if err != nil {
		t.Fatalf("Failed to marshal session data: %v", err)
	}

	err = os.WriteFile(sessionFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}

	// Test loading from file
	sessionData := lazyLoadSession(threadID)

	if sessionData == nil {
		t.Fatal("Expected session data from file, got nil")
	}

	if sessionData.ThreadID != expectedSession.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", sessionData.ThreadID, expectedSession.ThreadID)
	}

	if sessionData.SessionID != expectedSession.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", sessionData.SessionID, expectedSession.SessionID)
	}

	if sessionData.Model.ModelID != expectedSession.Model.ModelID {
		t.Errorf("Model ID mismatch: got %s, want %s", sessionData.Model.ModelID, expectedSession.Model.ModelID)
	}

	if len(sessionData.Commits) != len(expectedSession.Commits) {
		t.Errorf("Commits length mismatch: got %d, want %d", len(sessionData.Commits), len(expectedSession.Commits))
	}

	// Verify it was added to cache
	sessionMutex.RLock()
	cachedSession, exists := sessionCache[threadID]
	sessionMutex.RUnlock()

	if !exists {
		t.Error("Expected session to be cached after loading from file")
	}

	if cachedSession != sessionData {
		t.Error("Cached session should be the same object as returned")
	}
}

func TestLazyLoadSession_InvalidJSON(t *testing.T) {
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

	// Create temporary sessions directory
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, ".sessions")
	err := os.MkdirAll(sessionDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create sessions directory: %v", err)
	}

	// Mock sessionsDirectory global variable
	originalSessionsDirectory := sessionsDirectory
	sessionsDirectory = sessionDir
	defer func() {
		sessionsDirectory = originalSessionsDirectory
	}()

	threadID := "invalid-json-thread"

	// Write invalid JSON to file
	sessionFile := filepath.Join(sessionDir, threadID+".json")
	invalidJSON := `{"threadID": "test", "sessionID": incomplete json`

	err = os.WriteFile(sessionFile, []byte(invalidJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid session file: %v", err)
	}

	// Test loading invalid JSON should return nil
	sessionData := lazyLoadSession(threadID)

	if sessionData != nil {
		t.Error("Expected nil for invalid JSON file, got data")
	}
}

func TestEnsureSessionDir(t *testing.T) {
	// Save and reset global sessionsDirectory
	originalSessionsDirectory := sessionsDirectory
	sessionsDirectory = ""
	defer func() {
		sessionsDirectory = originalSessionsDirectory
	}()

	// Create a temporary directory for testing
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	sessionDir, err := ensureSessionDir()
	if err != nil {
		t.Fatalf("ensureSessionDir() failed: %v", err)
	}

	// Use filepath.EvalSymlinks to handle macOS /private symlinks
	expectedDir := filepath.Join(tempDir, ".sessions")
	expectedDirResolved, _ := filepath.EvalSymlinks(expectedDir)
	sessionDirResolved, _ := filepath.EvalSymlinks(sessionDir)

	if sessionDirResolved != expectedDirResolved {
		t.Errorf("Expected sessions directory %s, got %s", expectedDirResolved, sessionDirResolved)
	}

	// Check that directory was created
	info, err := os.Stat(sessionDir)
	if err != nil {
		t.Errorf("Sessions directory was not created: %v", err)
	}

	if !info.IsDir() {
		t.Error("Expected sessions directory to be a directory")
	}

	// Test that calling it again doesn't fail
	sessionDir2, err := ensureSessionDir()
	if err != nil {
		t.Errorf("Second call to ensureSessionDir() failed: %v", err)
	}

	if sessionDir != sessionDir2 {
		t.Error("ensureSessionDir should return consistent results")
	}
}

func TestSaveSessionData(t *testing.T) {
	// Save and reset global sessionsDirectory
	originalSessionsDirectory := sessionsDirectory
	sessionsDirectory = ""
	defer func() {
		sessionsDirectory = originalSessionsDirectory
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	// Create test session data
	threadID := "test-save-session"
	sessionData := &SessionData{
		ThreadID:       threadID,
		SessionID:      "session-save-123",
		Model:          Model{ProviderID: "openai", ModelID: "gpt-4"},
		WorktreePath:   "/tmp/test-worktree",
		RepositoryPath: "/tmp/test-repo",
		RepositoryName: "test-repo",
		CreatedAt:      time.Now(),
		Commits: []CommitRecord{
			{
				Hash:      "abc123",
				Summary:   "Test commit",
				Timestamp: time.Now(),
				Status:    "success",
			},
		},
	}

	// Test saving session data
	err := saveSessionData(sessionData)
	if err != nil {
		t.Fatalf("saveSessionData() failed: %v", err)
	}

	// Verify file was created
	sessionDir := filepath.Join(tempDir, ".sessions")
	sessionFile := filepath.Join(sessionDir, threadID+".json")

	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("Session file was not created")
	}

	// Verify file content by loading it back
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		t.Fatalf("Failed to read session file: %v", err)
	}

	var loadedSession SessionData
	err = json.Unmarshal(data, &loadedSession)
	if err != nil {
		t.Fatalf("Failed to unmarshal saved session: %v", err)
	}

	// Verify session data matches
	if loadedSession.ThreadID != sessionData.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", loadedSession.ThreadID, sessionData.ThreadID)
	}
	if loadedSession.SessionID != sessionData.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", loadedSession.SessionID, sessionData.SessionID)
	}
	if len(loadedSession.Commits) != len(sessionData.Commits) {
		t.Errorf("Commits length mismatch: got %d, want %d", len(loadedSession.Commits), len(sessionData.Commits))
	}
}

func TestCleanupSession(t *testing.T) {
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

	// Save and reset global sessionsDirectory
	originalSessionsDirectory := sessionsDirectory
	sessionsDirectory = ""
	defer func() {
		sessionsDirectory = originalSessionsDirectory
	}()

	// Create temporary directory
	tempDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	os.Chdir(tempDir)

	threadID := "test-cleanup-session"
	sessionData := &SessionData{
		ThreadID:  threadID,
		SessionID: "session-cleanup-123",
	}

	// Add to cache
	sessionMutex.Lock()
	sessionCache[threadID] = sessionData
	sessionMutex.Unlock()

	// Create session file
	err := saveSessionData(sessionData)
	if err != nil {
		t.Fatalf("Failed to save session data for test: %v", err)
	}

	// Verify session exists in cache and file
	sessionMutex.RLock()
	_, existsInCache := sessionCache[threadID]
	sessionMutex.RUnlock()
	if !existsInCache {
		t.Error("Session should exist in cache before cleanup")
	}

	sessionFile := filepath.Join(tempDir, ".sessions", threadID+".json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("Session file should exist before cleanup")
	}

	// Test cleanup
	err = CleanupSession(threadID)
	if err != nil {
		t.Errorf("CleanupSession() failed: %v", err)
	}

	// Verify session removed from cache
	sessionMutex.RLock()
	_, stillInCache := sessionCache[threadID]
	sessionMutex.RUnlock()
	if stillInCache {
		t.Error("Session should be removed from cache after cleanup")
	}

	// Verify session file removed
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("Session file should be removed after cleanup")
	}
}
