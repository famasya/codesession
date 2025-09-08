package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/sst/opencode-sdk-go"
)

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
	if client == nil {
		slog.Error("opencode client is nil", "thread_id", threadID)
		return nil
	}
	ctx := context.Background()

	// Enhanced message - add worktree boundary instruction for defense-in-depth
	enhancedMessage := message + "\n\nImportant: Stay within the current worktree directory for all file operations."

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
