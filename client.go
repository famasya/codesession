package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
)

var opencodeClient *opencode.Client
var worktreesDirectory string
var sessionsDirectory string

func ensureSessionDir() (string, error) {
	if sessionsDirectory != "" {
		return sessionsDirectory, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := fmt.Sprintf("%s/.sessions", cwd)
	if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
		return "", mkErr
	}
	sessionsDirectory = dir
	return dir, nil
}

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
		
		// Use ensureSessionDir helper
		_, err = ensureSessionDir()
		if err != nil {
			slog.Error("failed to ensure sessions directory", "error", err)
			return nil
		}

		slog.Debug("worktrees directory", "worktrees_directory", worktreesDirectory)
		slog.Debug("sessions directory", "sessions_directory", sessionsDirectory)

		// Create worktrees directory if it doesn't exist
		if err := os.MkdirAll(worktreesDirectory, 0755); err != nil {
			slog.Error("failed to create worktrees directory", "error", err)
			return nil
		}

		opencodeClient = opencode.NewClient(
			option.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", AppConfig.OpencodePort)),
		)
	}
	return opencodeClient
}
