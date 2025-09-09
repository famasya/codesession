package main

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
)

func SetLogLevel(levelStr string) {
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // default to info
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

type Config struct {
	BotToken              string       `json:"bot_token"`
	OpencodePort          int          `json:"opencode_port"`
	LogLevel              string       `json:"log_level"`
	SummarizerInstruction string       `json:"summarizer_instruction"`
	Repositories          []Repository `json:"repositories"`
	Models                []Model      `json:"models"`
}

type Repository struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type Model struct {
	ProviderID string `json:"provider_id"`
	ModelID    string `json:"model_id"`
}

var AppConfig Config

// stripComments removes // and /* */ comments from JSONC content
func stripComments(data []byte) []byte {
	var result []byte
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		// Remove // comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		// Remove /* */ comments (simple implementation)
		if idx := strings.Index(line, "/*"); idx != -1 {
			if endIdx := strings.Index(line[idx:], "*/"); endIdx != -1 {
				line = line[:idx] + line[idx+endIdx+2:]
			}
		}
		result = append(result, []byte(line+"\n")...)
	}
	return result
}

func LoadConfig() error {
	configFile := "config.jsonc"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		slog.Error("config.jsonc not found", "error", err)
		return err
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		slog.Error("failed to read config.jsonc", "error", err)
		return err
	}

	// Strip comments from JSONC
	cleanData := stripComments(data)

	err = json.Unmarshal(cleanData, &AppConfig)
	if err != nil {
		slog.Error("failed to decode config.jsonc", "error", err)
		return err
	}

	slog.Info("config loaded successfully")
	return nil
}
