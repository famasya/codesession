package main

import (
	"context"
	"sync"
	"time"

	"github.com/sst/opencode-sdk-go"
)

type MessagePartUpdated struct {
	Type       string `json:"type"`
	Properties struct {
		Part MessagePart `json:"part"`
	} `json:"properties"`
}

type MessagePart struct {
	ID        string `json:"id"`
	MessageID string `json:"messageID"`
	SessionID string `json:"sessionID"`
	Type      string `json:"type"`

	// Optional fields based on part type
	Text   string `json:"text,omitempty"`
	Tool   string `json:"tool,omitempty"`
	CallID string `json:"callID,omitempty"`

	// State field for tool parts
	State *ToolState `json:"state,omitempty"`

	// Time tracking
	Time *TimeRange `json:"time,omitempty"`

	// Tokens and cost info for step-finish parts
	Tokens *TokenInfo `json:"tokens,omitempty"`
	Cost   *float64   `json:"cost,omitempty"`
}

type ToolState struct {
	Status   string                 `json:"status"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   string                 `json:"output,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Time     *TimeRange             `json:"time,omitempty"`
}

type TimeRange struct {
	Start int64  `json:"start"`
	End   *int64 `json:"end,omitempty"`
}

type TokenInfo struct {
	Input     int       `json:"input"`
	Output    int       `json:"output"`
	Reasoning int       `json:"reasoning"`
	Cache     CacheInfo `json:"cache"`
}

type CacheInfo struct {
	Write int `json:"write"`
	Read  int `json:"read"`
}

// Part types identified from logs:
const (
	PartTypeText       = "text"        // User input text
	PartTypeReasoning  = "reasoning"   // AI reasoning/thinking
	PartTypeStepStart  = "step-start"  // Start of reasoning step
	PartTypeStepFinish = "step-finish" // End of reasoning step with tokens/cost
	PartTypeTool       = "tool"        // Tool execution (webfetch, etc.)
)

// Tool states observed:
const (
	ToolStatusPending   = "pending"
	ToolStatusRunning   = "running"
	ToolStatusCompleted = "completed"
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
