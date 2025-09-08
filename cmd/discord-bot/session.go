package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/sst/opencode-sdk-go"
)

// lazyLoadSession attempts to load a session from file for a specific threadID
func lazyLoadSession(threadID string) *SessionData {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	// Check if already in cache
	if sessionData, exists := sessionCache[threadID]; exists {
		return sessionData
	}

	// Ensure sessions directory exists
	sessionDir, err := ensureSessionDir()
	if err != nil {
		slog.Error("failed to ensure sessions directory", "error", err)
		return nil
	}

	// Try to load from file
	filePath := filepath.Join(sessionDir, fmt.Sprintf("%s.json", threadID))
	data, err := os.ReadFile(filePath)
	slog.Debug("lazy loading session from file", "thread_id", threadID, "file_path", filePath, "error", err)
	if err != nil {
		// File doesn't exist, no session to load
		return nil
	}

	var sessionData SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		slog.Error("failed to unmarshal session data", "thread_id", threadID, "error", err)
		return nil
	}

	// Use the sessionID from the file to connect to OpenCode
	// Note: We don't need to "restore" the session from server, just use the sessionID
	// The OpenCode server will handle the session, we just need to reference it

	// Create a mock session object with the sessionID from file
	session := &opencode.Session{
		ID: sessionData.SessionID,
	}

	// Store session with in-memory data (initially inactive)
	sessionData.Session = session
	sessionData.Active = false
	sessionCache[threadID] = &sessionData

	slog.Info("lazy loaded session", "thread_id", threadID, "session_id", session.ID)
	return &sessionData
}

// save session data to .sessions directory
func saveSessionData(sessionData *SessionData) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return err
	}

	sessionDir, err := ensureSessionDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(sessionDir, fmt.Sprintf("%s.json", sessionData.ThreadID))
	return os.WriteFile(filePath, data, 0644)
}

// get or create session for thread
func GetOrCreateSession(threadID, worktreePath, repositoryPath, repositoryName, userID string) *opencode.Session {
	client := Opencode()
	if client == nil {
		slog.Error("opencode client is nil", "thread_id", threadID)
		return nil
	}

	// Try to lazy load session first
	sessionData := lazyLoadSession(threadID)
	if sessionData != nil {
		slog.Info("using existing session", "thread_id", threadID)
		// Mark session as active
		sessionMutex.Lock()
		sessionData.Active = true
		sessionMutex.Unlock()
		return sessionData.Session
	}

	// Create new session
	ctx := context.Background()

	// Get absolute path for session creation
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		slog.Error("failed to get absolute path for session creation", "error", err)
		return nil
	}

	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.F(absWorktreePath),
	})
	if err != nil {
		slog.Error("failed to create session", "error", err)
		return nil
	}

	// Cache session with active state and save session data
	sessionData = &SessionData{
		ThreadID:       threadID,
		SessionID:      session.ID,
		Session:        session,
		Active:         true,
		WorktreePath:   absWorktreePath, // Store absolute path for consistency
		RepositoryPath: repositoryPath,
		RepositoryName: repositoryName,
		CreatedAt:      time.Now(),
		Commits:        make([]CommitRecord, 0),
		UserID:         userID,
	}
	sessionMutex.Lock()
	sessionCache[threadID] = sessionData
	sessionMutex.Unlock()

	if err := saveSessionData(sessionData); err != nil {
		slog.Error("failed to save session data", "error", err)
	}

	slog.Info("created new session", "thread_id", threadID, "session_id", session.ID)
	return session
}

// cleanup session (remove from cache and file)
func CleanupSession(threadID string) error {
	// Stop any active listener first
	stopActiveListener(threadID)

	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	delete(sessionCache, threadID)
	sessionDir, err := ensureSessionDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(sessionDir, fmt.Sprintf("%s.json", threadID))
	// remove only if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(filePath)
}

// set session active state by thread ID
func SetSessionActive(threadID string, active bool) *SessionData {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	if sessionData, exists := sessionCache[threadID]; exists {
		sessionData.Active = active
		return sessionData
	}
	return nil
}

// set session active state by session ID
func SetSessionActiveBySessionID(sessionID string, active bool) *SessionData {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	for _, sessionData := range sessionCache {
		if sessionData.Session != nil && sessionData.Session.ID == sessionID {
			sessionData.Active = active
			return sessionData
		}
	}
	return nil
}

// get session active state
func IsSessionActive(threadID string) bool {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	if sessionData, exists := sessionCache[threadID]; exists {
		return sessionData.Active
	}
	return false
}

// get session active state by session ID
func IsSessionActiveBySessionID(sessionID string) bool {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()

	for _, sessionData := range sessionCache {
		if sessionData.Session != nil && sessionData.Session.ID == sessionID {
			return sessionData.Active
		}
	}
	return false
}
