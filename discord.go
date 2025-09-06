package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

var discord *discordgo.Session
var mainWaitGroup *sync.WaitGroup
var mainContext context.Context

func RunDiscordBot(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Store references for message handlers
	mainWaitGroup = wg
	mainContext = ctx

	botToken := AppConfig.BotToken
	if botToken == "" {
		slog.Error("bot_token is not set in config.toml")
		return
	}

	discordSession, err := discordgo.New("Bot " + botToken)
	if err != nil {
		slog.Error("error creating Discord session", "error", err)
		return
	}
	discord = discordSession

	discord.AddHandler(InteractionHandlers)
	discord.AddHandler(MessageHandler)

	// We need both message events and application commands
	discord.Identify.Intents = discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = discord.Open()
	if err != nil {
		slog.Error("error opening connection", "error", err)
		return
	}

	slog.Info("discord bot started", "user", discord.State.User.Username)

	// Register slash commands
	err = registerCommands(discord)
	if err != nil {
		slog.Error("error registering commands", "error", err)
		return
	}

	// wait for ctx to be canceled
	<-ctx.Done()

	// Stop all active listeners before closing discord
	stopAllActiveListeners()
	discord.Close()
	slog.Info("discord bot stopped")
}

func registerCommands(s *discordgo.Session) error {
	repositoryList, err := repositoryList()
	if err != nil {
		return err
	}

	// choices
	var repositoryChoices []*discordgo.ApplicationCommandOptionChoice
	var modelChoices []*discordgo.ApplicationCommandOptionChoice
	for i, repository := range repositoryList {
		repositoryChoices = append(repositoryChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  repository.Name,
			Value: i,
		})
	}
	for i, model := range AppConfig.Models {
		modelChoices = append(modelChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  fmt.Sprintf("%s:%s", model.ProviderID, model.ModelID),
			Value: i,
		})
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Will reply you back",
		},
		{
			Name:        "commit",
			Description: "Generate commit message push changes",
		},
		{
			Name:        "opencode",
			Description: "Starting work with Opencode",
			Type:        discordgo.ChatApplicationCommand,
			Options: []*discordgo.ApplicationCommandOption{
				{
					Name:        "repository",
					Description: "Select repository",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
					Choices:     repositoryChoices,
				},
				{
					Name:        "model",
					Description: "Select model",
					Type:        discordgo.ApplicationCommandOptionInteger,
					Required:    true,
					Choices:     modelChoices,
				},
			},
		},
	}

	for _, command := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, "", command)
		if err != nil {
			return err
		}
	}

	slog.Info("slash commands registered successfully")
	return nil
}

func repositoryList() ([]Repository, error) {
	var repositoryList []Repository
	// check if directory exists and is a git repository
	for _, repository := range AppConfig.Repositories {
		if _, err := os.Stat(repository.Path); os.IsNotExist(err) {
			slog.Error("repository directory not found", "path", repository.Path, "error", err)
			return nil, err
		}

		// check if directory is a git repository
		if _, err := os.Stat(repository.Path + "/.git"); os.IsNotExist(err) {
			slog.Error("repository is not a git repository", "path", repository.Path, "error", err)
			return nil, err
		}
	}

	// print repository list
	for _, repository := range AppConfig.Repositories {
		slog.Debug("repository found", "path", repository.Path, "name", repository.Name)
		repositoryList = append(repositoryList, repository)
	}

	return repositoryList, nil
}

const limit = 2000

// send message to discord and chunk if necessarry
func SendDiscordMessage(threadID string, message string) {
	remaining := message
	for len(remaining) > 0 {
		chunk := remaining
		if len(chunk) > limit {
			split := strings.LastIndex(chunk[:limit], "\n")
			if split <= 0 {
				split = limit
			}
			chunk = remaining[:split]
			remaining = strings.TrimPrefix(remaining[split:], "\n")
		} else {
			remaining = ""
		}
		if _, err := discord.ChannelMessageSend(threadID, chunk); err != nil {
			slog.Error("failed to send message to discord", "thread_id", threadID, "error", err)
			break
		}
		slog.Debug("sent message chunk to discord", "thread_id", threadID, "chunk_len", len(chunk))
	}
}
