package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/sst/opencode-sdk-go"
)

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
			// Ensure streaming state is set when SSE connects
			sessionMutex.Lock()
			if sessionData, exists := sessionCache[threadID]; exists {
				sessionData.IsStreaming = true
				slog.Debug("confirmed session as streaming", "thread_id", threadID)
			}
			sessionMutex.Unlock()
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
			switch part.Type {
			case PartTypeTool:
				// for tool parts, only send completed tools as status updates
				if part.Tool != "" && part.State != nil && part.State.Status == ToolStatusCompleted {
					toolUpdate := fmt.Sprintf("|>> tool: %s", part.Tool)
					updateToolStatus(threadID, toolUpdate)
				}
			case PartTypeReasoning:
				if part.Text != "" {
					reasoningUpdate := fmt.Sprintf("|>> thinking: %s", part.Text)
					updateToolStatus(threadID, reasoningUpdate)
				}
			case PartTypeText:
				// Text responses should be sent as status updates to maintain chronological order
				if part.Text != "" {
					cleanText := fmt.Sprintf("Response:\n%s", removeExcessiveNewLine(part.Text))
					updateTextResponse(threadID, cleanText)
				}
			}

			// debug log
			slog.Debug("processing message for Discord", "thread_id", threadID, "session_id", sessionData.SessionID, "part_type", part.Type)
		case opencode.EventListResponseTypeSessionIdle:
			eventData := serializeEvent[struct {
				SessionID string `json:"sessionId"`
			}](&event)
			if eventData == nil {
				slog.Error("failed to serialize session idle event")
				continue
			}

			slog.Debug("session idle detected", "thread_id", threadID, "session_id", eventData.SessionID)

			// Mark session as no longer streaming (completed)
			sessionMutex.Lock()
			if sessionData, exists := sessionCache[threadID]; exists {
				sessionData.IsStreaming = false
				slog.Debug("marked session as not streaming", "thread_id", threadID)
			}
			sessionMutex.Unlock()

			// Mention the user that the task is completed (keep existing text responses intact)
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
