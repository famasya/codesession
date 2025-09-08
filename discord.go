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
			Name:        "diff",
			Description: "Show diff of changes in current worktree",
		},
		{
			Name:        "codesession",
			Description: "Start new codesession",
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

// send message to discord and chunk if necessarry
const messageLimit = 2000

// send diff message to discord with proper code block formatting for each chunk
func SendDiscordDiffMessage(threadID string, diffOutput string) {
	remaining := diffOutput
	for len(remaining) > 0 {
		chunk := remaining
		// Account for code block wrapper length (```diff\n and \n```)
		maxContent := messageLimit - 10
		if len(chunk) > maxContent {
			split := strings.LastIndex(chunk[:maxContent], "\n")
			if split <= 0 {
				split = maxContent
			}
			chunk = remaining[:split]
			remaining = strings.TrimPrefix(remaining[split:], "\n")
		} else {
			remaining = ""
		}

		// Wrap each chunk in diff code block
		wrappedChunk := fmt.Sprintf("```diff\n%s\n```", chunk)

		if _, err := discord.ChannelMessageSend(threadID, wrappedChunk); err != nil {
			slog.Error("failed to send diff message to discord", "thread_id", threadID, "error", err)
			break
		}
		slog.Debug("sent diff chunk to discord", "thread_id", threadID, "chunk_len", len(wrappedChunk))
	}
}

func SendDiscordMessage(threadID string, message string) {
	remaining := message
	for len(remaining) > 0 {
		chunk := remaining
		if len(chunk) > messageLimit {
			split := strings.LastIndex(chunk[:messageLimit], "\n")
			if split <= 0 {
				split = messageLimit
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

// updateToolStatus appends tool status updates (formatted as blockquotes)
func updateToolStatus(threadID, toolUpdate string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionData, exists := sessionCache[threadID]
	if !exists {
		slog.Error("session not found for tool status update", "thread_id", threadID)
		return
	}

	// Format as blockquote and append to tool status history
	formattedUpdate := formatBlockquote(toolUpdate)
	sessionData.ToolStatusHistory = appendToContentHistory(sessionData.ToolStatusHistory, formattedUpdate)

	// Rebuild and update the complete message
	rebuildStatusMessage(threadID, sessionData)
}

// updateTextResponse replaces the current response content
func updateTextResponse(threadID, textResponse string) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()

	sessionData, exists := sessionCache[threadID]
	if !exists {
		slog.Error("session not found for text response update", "thread_id", threadID)
		return
	}

	// Replace the current response content (not append, replace for new responses)
	sessionData.CurrentResponse = textResponse

	// Rebuild and update the complete message
	rebuildStatusMessage(threadID, sessionData)
}

// rebuildStatusMessage combines content history and updates Discord message
func rebuildStatusMessage(threadID string, sessionData *SessionData) {
	const maxMessageLength = 1800 // Leave buffer before Discord's 2000 limit

	// Build the complete message content
	header := "```fix\n✨codesession is working...\n```"
	var parts []string

	// Add tool status history if present
	if sessionData.ToolStatusHistory != "" {
		parts = append(parts, sessionData.ToolStatusHistory)
	}

	// Add current response if present
	if sessionData.CurrentResponse != "" {
		parts = append(parts, sessionData.CurrentResponse)
	}

	// Combine all parts
	newContent := header
	if len(parts) > 0 {
		newContent += "\n" + strings.Join(parts, "\n")
	}

	// Check if we need to create a continuation message
	if len(newContent) > maxMessageLength {
		// Mark current message as continued
		if sessionData.LastStatusMessageID != "" {
			continuedContent := sessionData.StatusMessageContent + "\n...continued below..."
			editDiscordMessage(threadID, sessionData.LastStatusMessageID, continuedContent)
		}

		// Calculate how much content we can fit in continuation message
		continueHeader := "```fix\n✨codesession is working (continued...)\n```\n"
		maxContentForContinuation := maxMessageLength - len(continueHeader)

		// Combine parts and truncate if needed
		combinedContent := strings.Join(parts, "\n")
		truncatedContent := combinedContent
		if len(truncatedContent) > maxContentForContinuation {
			truncatedContent = truncatedContent[len(truncatedContent)-maxContentForContinuation:]
			// Try to start from a newline to avoid cutting mid-line
			if newlineIndex := strings.Index(truncatedContent, "\n"); newlineIndex != -1 {
				truncatedContent = truncatedContent[newlineIndex+1:]
			}
		}

		// Create new continuation message
		newStatusContent := continueHeader + truncatedContent
		msg, err := discord.ChannelMessageSend(threadID, newStatusContent)
		if err != nil {
			slog.Error("failed to create continuation status message", "thread_id", threadID, "error", err)
			return
		}

		sessionData.LastStatusMessageID = msg.ID
		sessionData.StatusMessageContent = newStatusContent
		slog.Debug("created continuation status message", "thread_id", threadID, "message_id", msg.ID)
		return
	}

	// Update existing message or create new one
	if sessionData.LastStatusMessageID == "" {
		// Create initial status message
		msg, err := discord.ChannelMessageSend(threadID, newContent)
		if err != nil {
			slog.Error("failed to create initial status message", "thread_id", threadID, "error", err)
			return
		}
		sessionData.LastStatusMessageID = msg.ID
		sessionData.StatusMessageContent = newContent
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
