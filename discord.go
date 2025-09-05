package main

import (
	"context"
	"log/slog"
	"os"
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
			Name:  model,
			Value: i,
		})
	}

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "ping",
			Description: "Will reply you back",
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
