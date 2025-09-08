package main

import (
	"context"
	"sync"
	"time"

	"github.com/sst/opencode-sdk-go"
)

// CommitRecord represents a git commit with metadata
type CommitRecord struct {
	Hash      string    `json:"hash"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "success", "failed", "pending"
}

// SessionData holds all information about an OpenCode session
type SessionData struct {
	ThreadID       string         `json:"thread_id"`
	SessionID      string         `json:"session_id"`
	Model          Model          `json:"model"`
	WorktreePath   string         `json:"worktree_path"`
	RepositoryPath string         `json:"repository_path"`
	RepositoryName string         `json:"repository_name"`
	CreatedAt      time.Time      `json:"created_at"`
	Commits        []CommitRecord `json:"commits"`

	// Non-serialized runtime fields
	Session              *opencode.Session `json:"-"` // Don't serialize the session object
	Active               bool              `json:"-"` // Don't serialize the active state
	IsStreaming          bool              `json:"-"` // Don't serialize the SSE streaming state
	LastStatusMessageID  string            `json:"-"` // Don't serialize the last status message ID
	StatusMessageContent string            `json:"-"` // Don't serialize the current status message content
	ToolStatusHistory    string            `json:"-"` // Don't serialize the tool/thinking status history
	CurrentResponse      string            `json:"-"` // Don't serialize the current text response
	UserID               string            `json:"-"` // Don't serialize the user ID who started the session
}

// Global variables for session management
var sessionCache = make(map[string]*SessionData, 100) // Pre-allocate for typical load
var sessionMutex sync.RWMutex

// Active event listeners management
var activeListeners = make(map[string]context.CancelFunc, 100) // Pre-allocate for typical load
var listenersMutex sync.RWMutex
