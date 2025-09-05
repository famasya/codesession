package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/sst/opencode-sdk-go" // imported as opencode
	"github.com/sst/opencode-sdk-go/option"
)

var opencodeClient *opencode.Client
var worktreesDirectory string
var sessionsDirectory string

type SessionData struct {
	ThreadID  string            `json:"thread_id"`
	SessionID string            `json:"session_id"`
	Session   *opencode.Session `json:"-"` // Don't serialize the session object
	Active    bool              `json:"-"` // Don't serialize the active state
}

var sessionCache = make(map[string]*SessionData)
var sessionMutex sync.RWMutex

// setup opencode singleton
func Opencode() *opencode.Client {
	if opencodeClient == nil {
		// set current directory
		currentDir, err := os.Getwd()
		if err != nil {
			slog.Error("failed to get current directory", "error", err)
			return nil
		}
		worktreesDirectory = fmt.Sprintf("%s/.worktrees", currentDir)
		sessionsDirectory = fmt.Sprintf("%s/.sessions", currentDir)

		// Create sessions directory if it doesn't exist
		if err := os.MkdirAll(sessionsDirectory, 0755); err != nil {
			slog.Error("failed to create sessions directory", "error", err)
			return nil
		}

		opencodeClient = opencode.NewClient(
			option.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", AppConfig.OpencodePort)),
		)
		loadSessions()
	}
	return opencodeClient
}

// load existing sessions from .sessions directory
func loadSessions() {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	files, err := os.ReadDir(sessionsDirectory)
	if err != nil {
		slog.Error("failed to read sessions directory", "error", err)
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			filePath := filepath.Join(sessionsDirectory, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				slog.Error("failed to read session file", "file", file.Name(), "error", err)
				continue
			}

			var sessionData SessionData
			if err := json.Unmarshal(data, &sessionData); err != nil {
				slog.Error("failed to unmarshal session data", "file", file.Name(), "error", err)
				continue
			}

			// Try to restore session
			ctx := context.Background()
			session, err := opencodeClient.Session.Get(ctx, sessionData.SessionID, opencode.SessionGetParams{})
			if err != nil {
				slog.Warn("failed to restore session, will create new one", "thread_id", sessionData.ThreadID, "error", err)
				// Remove invalid session file
				os.Remove(filePath)
				continue
			}

			// Store session with in-memory data (initially inactive)
			sessionData.Session = session
			sessionData.Active = false
			sessionCache[sessionData.ThreadID] = &sessionData

			slog.Debug("restored session", "thread_id", sessionData.ThreadID, "session_id", session.ID)
		}
	}
}

// save session data to .sessions directory
func saveSessionData(sessionData *SessionData) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	data, err := json.MarshalIndent(sessionData, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", sessionData.ThreadID))
	return os.WriteFile(filePath, data, 0644)
}

// get or create session for thread
func GetOrCreateSession(threadID, worktreePath string) *opencode.Session {
	// Ensure OpenCode client is initialized
	client := Opencode()
	if client == nil {
		slog.Error("failed to initialize opencode client")
		return nil
	}

	sessionMutex.RLock()
	sessionData, exists := sessionCache[threadID]
	sessionMutex.RUnlock()

	if exists {
		slog.Info("using existing session", "thread_id", threadID)
		// Mark session as active
		sessionMutex.Lock()
		sessionData.Active = true
		sessionMutex.Unlock()
		return sessionData.Session
	}

	// Create new session
	ctx := context.Background()
	session, err := client.Session.New(ctx, opencode.SessionNewParams{
		Directory: opencode.F(worktreePath),
	})
	if err != nil {
		slog.Error("failed to create session", "error", err)
		return nil
	}

	// Cache session with active state and save session data
	sessionData = &SessionData{
		ThreadID:  threadID,
		SessionID: session.ID,
		Session:   session,
		Active:    true,
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

// send message to session
func SendMessage(threadID string, message string) *opencode.SessionPromptResponse {
	session := GetOrCreateSession(threadID, fmt.Sprintf("%s/%s", worktreesDirectory, threadID))
	if session == nil {
		return nil
	}

	// Ensure OpenCode client is initialized
	client := Opencode()
	if client == nil {
		slog.Error("failed to initialize opencode client")
		return nil
	}

	ctx := context.Background()
	response, err := client.Session.Prompt(ctx, session.ID, opencode.SessionPromptParams{
		Directory: opencode.F(fmt.Sprintf("%s/%s", worktreesDirectory, threadID)),
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			&opencode.TextPartInputParam{
				Type: opencode.F(opencode.TextPartInputTypeText),
				Text: opencode.F(message),
			},
		}),
		Model: opencode.F(opencode.SessionPromptParamsModel{
			ProviderID: opencode.F("opencode"),  // change this later
			ModelID:    opencode.F("grok-code"), // change this later
		}),
	})
	if err != nil {
		slog.Error("failed to send message", "error", err)
		return nil
	}

	return response
}

// cleanup session (remove from cache and file)
func CleanupSession(threadID string) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	delete(sessionCache, threadID)
	filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", threadID))
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
		if sessionData.Session.ID == sessionID {
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
		if sessionData.Session.ID == sessionID {
			return sessionData.Active
		}
	}
	return false
}

func CleanupWorktree(threadID string) error {
	// cleanup using git commands
	cmd := exec.Command("git", "worktree", "remove", fmt.Sprintf("%s/%s", worktreesDirectory, threadID))
	return cmd.Run()
}
