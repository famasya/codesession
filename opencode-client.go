package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sst/opencode-sdk-go" // imported as opencode
	"github.com/sst/opencode-sdk-go/option"
)

var opencodeClient *opencode.Client
var worktreesDirectory string
var sessionsDirectory string

type CommitRecord struct {
	Hash      string    `json:"hash"`
	Summary   string    `json:"summary"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"` // "success", "failed", "pending"
}

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
	LastStatusMessageID  string            `json:"-"` // Don't serialize the last status message ID
	StatusMessageContent string            `json:"-"` // Don't serialize the current status message content
	UserID               string            `json:"-"` // Don't serialize the user ID who started the session
}

var sessionCache = make(map[string]*SessionData)
var sessionMutex sync.RWMutex

// Active event listeners management
var activeListeners = make(map[string]context.CancelFunc)
var listenersMutex sync.RWMutex

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

		slog.Debug("worktrees directory", "worktrees_directory", worktreesDirectory)
		slog.Debug("sessions directory", "sessions_directory", sessionsDirectory)

		// Create worktrees directory if it doesn't exist
		if err := os.MkdirAll(worktreesDirectory, 0755); err != nil {
			slog.Error("failed to create worktrees directory", "error", err)
			return nil
		}

		// Create sessions directory if it doesn't exist
		if err := os.MkdirAll(sessionsDirectory, 0755); err != nil {
			slog.Error("failed to create sessions directory", "error", err)
			return nil
		}

		opencodeClient = opencode.NewClient(
			option.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", AppConfig.OpencodePort)),
		)
	}
	return opencodeClient
}

// lazyLoadSession attempts to load a session from file for a specific threadID
func lazyLoadSession(threadID string) *SessionData {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	// Check if already in cache
	if sessionData, exists := sessionCache[threadID]; exists {
		return sessionData
	}

	// Try to load from file
	filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", threadID))
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

	filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", sessionData.ThreadID))
	return os.WriteFile(filePath, data, 0644)
}

// get or create session for thread
func GetOrCreateSession(threadID, worktreePath, repositoryPath, repositoryName, userID string) *opencode.Session {
	client := Opencode()

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

// send message to session
func SendMessage(threadID string, message string) *opencode.SessionPromptResponse {
	sessionMutex.RLock()
	sessionData, exists := sessionCache[threadID]
	sessionMutex.RUnlock()
	if !exists {
		return nil
	}

	// Use the session's stored worktree path and existing session
	model := sessionData.Model
	session := sessionData.Session
	worktreePath := sessionData.WorktreePath

	if session == nil {
		slog.Error("session object is nil for thread", "thread_id", threadID)
		return nil
	}

	slog.Debug("sending message to session", "thread_id", threadID, "session_id", session.ID, "message", message, "worktree_path", worktreePath)

	// Validate that the worktree path exists and is accessible
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		slog.Error("worktree path does not exist", "thread_id", threadID, "worktree_path", worktreePath)
		return nil
	}

	// Get absolute path to ensure OpenCode SDK gets the correct directory
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		slog.Error("failed to get absolute path for worktree", "thread_id", threadID, "worktree_path", worktreePath, "error", err)
		return nil
	}
	slog.Debug("using absolute worktree path", "thread_id", threadID, "abs_worktree_path", absWorktreePath)

	client := Opencode()
	ctx := context.Background()

	// Enhanced message - just pass the user request without confusing path information
	enhancedMessage := message

	response, err := client.Session.Prompt(ctx, session.ID, opencode.SessionPromptParams{
		Directory: opencode.F(absWorktreePath),
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			&opencode.TextPartInputParam{
				Type: opencode.F(opencode.TextPartInputTypeText),
				Text: opencode.F(enhancedMessage),
			},
		}),
		Model: opencode.F(opencode.SessionPromptParamsModel{
			ProviderID: opencode.F(model.ProviderID),
			ModelID:    opencode.F(model.ModelID),
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
	// Stop any active listener first
	stopActiveListener(threadID)

	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	delete(sessionCache, threadID)
	filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", threadID))
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
	// Prefer repoPath from session; otherwise infer from known worktree layout
	sessionData := lazyLoadSession(threadID)
	var worktreePath, repoPath string
	if sessionData != nil && sessionData.RepositoryPath != "" {
		repoPath = sessionData.RepositoryPath
		worktreePath = sessionData.WorktreePath
	} else {
		worktreePath = filepath.Join(worktreesDirectory, threadID)
		// When we don't have session data, we can't determine the repo path
		// This should not happen in normal operations since sessions store repo path
		return fmt.Errorf("cannot determine repository path for cleanup without session data for thread %s", threadID)
	}
	return gitOps.RemoveWorktree(repoPath, worktreePath)
}

func OpencodeEventsListener(ctx context.Context, wg *sync.WaitGroup, threadID string) {
	defer func() {
		wg.Done()
		slog.Debug("workgroup for OpencodeEventsListener released", "thread_id", threadID)
	}()

	// Get session data for this thread
	sessionMutex.RLock()
	sessionData, exists := sessionCache[threadID]
	sessionMutex.RUnlock()

	if !exists {
		slog.Error("session not found for thread", "thread_id", threadID)
		return
	}

	// Use stored worktree path from session data
	worktreePath := sessionData.WorktreePath
	client := Opencode()
	stream := client.Event.ListStreaming(ctx, opencode.EventListParams{
		Directory: opencode.F(worktreePath),
	})

	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case opencode.EventListResponseTypeServerConnected:
			slog.Debug("started session event listener", "thread_id", threadID, "session_id", sessionData.SessionID)
		case opencode.EventListResponseTypeMessagePartUpdated:
			// Parse the event directly from raw JSON properties
			eventData := serializeEvent[struct {
				Part MessagePart `json:"part"`
			}](&event)
			if eventData == nil {
				slog.Error("failed to serialize message part updated event")
				continue
			}

			// for tool parts, only send completed tools to Discord
			// for other parts (text, reasoning), send them regardless of time
			part := eventData.Part
			shouldSendToDiscord := false
			if part.Type == PartTypeTool {
				// for tools, check time in the state field (not part.Time)
				if part.State != nil && part.State.Status == ToolStatusCompleted && part.State.Time != nil && part.State.Time.End != nil {
					shouldSendToDiscord = true
				}
			} else {
				// for non-tool parts (text, reasoning), send if time.end is present
				if part.Time != nil && part.Time.End != nil {
					shouldSendToDiscord = true
				}
			}

			if !shouldSendToDiscord {
				// skip if not ready for discord
				continue
			}

			// format message based on part type
			var statusUpdate string
			var shouldCreateNewMessage bool
			
			switch part.Type {
			case PartTypeTool:
				// for tool parts, only send completed tools as status updates
				if part.Tool != "" && part.State != nil && part.State.Status == ToolStatusCompleted {
					statusUpdate = fmt.Sprintf("Tool: %s", part.Tool)
				}
			case PartTypeReasoning:
				if part.Text != "" {
					statusUpdate = "*" + part.Text + "*"
				}
			case PartTypeText:
				// Text responses should be sent as new messages, not status updates
				if part.Text != "" {
					cleanText := removeExcessiveNewLine(part.Text)
					sendToDiscord(threadID, cleanText)
					shouldCreateNewMessage = true
				}
			}

			// debug log
			slog.Debug("processing message for Discord", "thread_id", threadID, "session_id", sessionData.SessionID, "status_update", statusUpdate, "new_message", shouldCreateNewMessage)

			// update status message if we have a status update
			if statusUpdate != "" && !shouldCreateNewMessage {
				// Format the status update as blockquote to handle multi-line content
				formattedUpdate := formatBlockquote(statusUpdate)
				updateStatusMessage(threadID, formattedUpdate)
			}
		case opencode.EventListResponseTypeSessionIdle:
			eventData := serializeEvent[struct {
				SessionID string `json:"sessionId"`
			}](&event)
			if eventData == nil {
				slog.Error("failed to serialize session idle event")
				continue
			}

			slog.Debug("session idle detected", "thread_id", threadID, "session_id", eventData.SessionID)

			// Mark status message as completed
			updateStatusMessage(threadID, "Task completed!")

			// Mention the user that the task is completed
			sessionMutex.RLock()
			sessionData, exists := sessionCache[threadID]
			sessionMutex.RUnlock()
			if exists && sessionData.UserID != "" {
				mentionMessage := fmt.Sprintf("<@%s> Task completed!", sessionData.UserID)
				sendToDiscord(threadID, mentionMessage)
			}

			// set session inactive and cleanup
			SetSessionActive(threadID, false)

			// remove from active listeners and exit
			removeActiveListener(threadID)
			return
		}
	}

	if err := stream.Err(); err != nil {
		slog.Error("error in opencode event stream", "thread_id", threadID, "error", err)
	}

	// Cleanup on exit
	removeActiveListener(threadID)
	slog.Debug("opencode events listener stopped", "thread_id", threadID)
}

func serializeEvent[T any](event *opencode.EventListResponse) *T {
	var data T
	err := json.Unmarshal([]byte(event.JSON.Properties.Raw()), &data)
	if err != nil {
		slog.Error("failed to serialize event to json", "error", err)
		return nil
	}
	return &data
}

// removeActiveListener removes the cancel function for a session listener
func removeActiveListener(threadID string) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()
	delete(activeListeners, threadID)
}

// stopActiveListener cancels and removes a listener for a thread
func stopActiveListener(threadID string) {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()
	if cancelFunc, exists := activeListeners[threadID]; exists {
		cancelFunc()
		delete(activeListeners, threadID)
		slog.Debug("stopped active listener", "thread_id", threadID)
	}
}

// stopAllActiveListeners cancels and removes all active listeners (for shutdown)
func stopAllActiveListeners() {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()
	for threadID, cancelFunc := range activeListeners {
		cancelFunc()
		slog.Debug("stopped active listener", "thread_id", threadID)
	}
	// Clear the map
	activeListeners = make(map[string]context.CancelFunc)
	slog.Info("stopped all active listeners")
}

// spawnListenerIfNotExists atomically checks and spawns a listener for a thread
// Returns true if a new listener was spawned, false if one already exists
func spawnListenerIfNotExists(ctx context.Context, wg *sync.WaitGroup, threadID string) bool {
	listenersMutex.Lock()
	defer listenersMutex.Unlock()

	// Check if listener already exists
	if _, exists := activeListeners[threadID]; exists {
		return false // Already exists
	}

	// Create child context for this listener
	listenerCtx, cancel := context.WithCancel(ctx)

	// Add to waitgroup and register listener
	wg.Add(1)
	activeListeners[threadID] = cancel

	// Start listener
	go OpencodeEventsListener(listenerCtx, wg, threadID)
	slog.Debug("spawned session event listener", "thread_id", threadID)

	return true // New listener spawned
}

// formatBlockquote adds blockquote to text
func formatBlockquote(text string) string {
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" { // skip blank lines
			out = append(out, "> "+line)
		}
	}
	return strings.Join(out, "\n")
}

func removeExcessiveNewLine(text string) string {
	// remove excessive to only 1 new line
	text = regexp.MustCompile(`\n\n+`).ReplaceAllString(text, "\n\n")
	return text
}

// editDiscordMessage edits an existing Discord message
func editDiscordMessage(threadID, messageID, newContent string) error {
	if discord == nil {
		slog.Error("discord session not available", "thread_id", threadID)
		return fmt.Errorf("discord session not available")
	}

	_, err := discord.ChannelMessageEdit(threadID, messageID, newContent)
	if err != nil {
		slog.Error("failed to edit message on discord", "thread_id", threadID, "message_id", messageID, "error", err)
		return err
	} else {
		slog.Debug("edited message on discord", "thread_id", threadID, "message_id", messageID, "content_length", len(newContent))
	}
	return nil
}

// updateStatusMessage appends a new status update to the current status message
// Handles character limits by creating continuation messages when needed
func updateStatusMessage(threadID, statusUpdate string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionData, exists := sessionCache[threadID]
	if !exists {
		slog.Error("session not found for status update", "thread_id", threadID)
		return
	}

	const maxMessageLength = 1800 // Leave buffer before Discord's 2000 limit
	newContent := sessionData.StatusMessageContent + "\n" + statusUpdate

	// Check if we need to create a continuation message
	if len(newContent) > maxMessageLength {
		// Mark current message as continued
		continuedContent := sessionData.StatusMessageContent + "\n...continued below..."
		if sessionData.LastStatusMessageID != "" {
			editDiscordMessage(threadID, sessionData.LastStatusMessageID, continuedContent)
		}

		// Create new continuation message
		newStatusContent := "CodeSession Working (continued)...\n...continued from above...\n" + statusUpdate
		msg, err := discord.ChannelMessageSend(threadID, newStatusContent)
		if err != nil {
			slog.Error("failed to create continuation status message", "thread_id", threadID, "error", err)
			return
		}

		// Update session data with new message
		sessionData.LastStatusMessageID = msg.ID
		sessionData.StatusMessageContent = newStatusContent
		slog.Debug("created continuation status message", "thread_id", threadID, "message_id", msg.ID)
	} else {
		// Update existing message
		if sessionData.LastStatusMessageID == "" {
			// Create initial status message
			initialContent := "CodeSession Working...\n" + statusUpdate
			msg, err := discord.ChannelMessageSend(threadID, initialContent)
			if err != nil {
				slog.Error("failed to create initial status message", "thread_id", threadID, "error", err)
				return
			}
			sessionData.LastStatusMessageID = msg.ID
			sessionData.StatusMessageContent = initialContent
			slog.Debug("created initial status message", "thread_id", threadID, "message_id", msg.ID)
		} else {
			// Edit existing message
			sessionData.StatusMessageContent = newContent
			err := editDiscordMessage(threadID, sessionData.LastStatusMessageID, newContent)
			if err != nil {
				slog.Error("failed to update status message", "thread_id", threadID, "error", err)
			}
		}
	}
}

// sendToDiscord sends a message to the Discord channel
func sendToDiscord(threadID, message string) {
	if discord == nil {
		slog.Error("discord session not available", "thread_id", threadID)
		return
	}

	_, err := discord.ChannelMessageSend(threadID, message)
	if err != nil {
		slog.Error("failed to send message to discord", "thread_id", threadID, "error", err)
	} else {
		slog.Debug("sent message to discord", "thread_id", threadID, "message_length", len(message))
	}
}
