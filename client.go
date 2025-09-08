package main

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/sst/opencode-sdk-go"
	"github.com/sst/opencode-sdk-go/option"
)

var opencodeClient *opencode.Client
var opencodeOnce sync.Once
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
	opencodeOnce.Do(func() {
		// Use ensureSessionDir helper
		_, err := ensureSessionDir()
		if err != nil {
			slog.Error("failed to ensure sessions directory", "error", err)
			return
		}

		slog.Debug("sessions directory", "sessions_directory", sessionsDirectory)

		opencodeClient = opencode.NewClient(
			option.WithBaseURL(fmt.Sprintf("http://127.0.0.1:%d", AppConfig.OpencodePort)),
		)
	})
	return opencodeClient
}
