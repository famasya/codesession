package main

import (
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	BotToken             string       `toml:"bot_token"`
	OpencodePort         int          `toml:"opencode_port"`
	LogLevel             string       `toml:"log_level"`
	SummarizerInstruction string      `toml:"summarizer_instruction"`
	Repositories         []Repository `toml:"repositories"`
	Models               []Model      `toml:"models"`
}

type Repository struct {
	Path string `toml:"path"`
	Name string `toml:"name"`
}

type Model struct {
	ProviderID string `toml:"provider_id"`
	ModelID    string `toml:"model_id"`
}

var AppConfig Config

func LoadConfig() error {
	configFile := "config.toml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		slog.Error("config.toml not found", "error", err)
		return err
	}

	_, err := toml.DecodeFile(configFile, &AppConfig)
	if err != nil {
		slog.Error("failed to decode config.toml", "error", err)
		return err
	}

	slog.Info("config loaded successfully")
	return nil
}
