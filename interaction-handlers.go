package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/goombaio/namegenerator"
)

var seed = time.Now().UnixNano()
var generator = namegenerator.NewNameGenerator(seed)

func InteractionHandlers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	command := i.ApplicationCommandData().Name
	if command == "ping" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Pong!",
			},
		})
	}

	if command == "opencode" {
		handleOpencodeCommand(s, i)
	}
}

func handleOpencodeCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Respond immediately to prevent timeout
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		slog.Error("failed to respond to interaction", "error", err)
		return
	}

	// Get command options
	options := i.ApplicationCommandData().Options
	var repositoryIndex, modelIndex int

	for _, option := range options {
		switch option.Name {
		case "repository":
			repositoryIndex = int(option.IntValue())
		case "model":
			modelIndex = int(option.IntValue())
		}
	}

	// Get selected repository
	if repositoryIndex >= len(AppConfig.Repositories) {
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Invalid repository selection"}[0],
		})
		return
	}

	repository := AppConfig.Repositories[repositoryIndex]
	model := AppConfig.Models[modelIndex]

	// Create a new thread
	threadName := generator.Generate()
	thread, err := s.ThreadStart(
		i.ChannelID,
		fmt.Sprintf("OpenCode: %s", threadName),
		discordgo.ChannelTypeGuildPublicThread,
		1440, // 24 hours
	)
	if err != nil {
		slog.Error("failed to create thread", "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create thread"}[0],
		})
		return
	}

	// Create worktree directory
	worktreeDir := filepath.Join(repository.Path, ".worktrees", thread.ID)
	err = os.MkdirAll(filepath.Dir(worktreeDir), 0755)
	if err != nil {
		slog.Error("failed to create worktrees directory", "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create worktrees directory"}[0],
		})
		return
	}

	// Create git worktree and create new session
	cmd := exec.Command("git", "worktree", "add", worktreeDir)
	cmd.Dir = repository.Path

	// Create session
	session := GetOrCreateSession(thread.ID, worktreeDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to create git worktree", "error", err, "output", string(output))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create git worktree"}[0],
		})
		return
	}

	// Send initial message to the thread
	trimmedWorktreeDir := strings.TrimPrefix(worktreeDir, repository.Path)
	trimmedSessionID := session.ID[len(session.ID)-8:]
	s.ChannelMessageSend(thread.ID, fmt.Sprintf("```\nOpenCode Session Started\nRepository: %s\nModel: %s\nWorktree Path: `%s`\nSession ID: %s\n```",
		repository.Name, model, trimmedWorktreeDir, trimmedSessionID))

	// Update the interaction response with success message
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &[]string{fmt.Sprintf("OpenCode session created successfully! Check the thread: %s", thread.Mention())}[0],
	})
}

func MessageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore messages from the bot itself
	if m.Author.ID == s.State.User.ID {
		return
	}

	// Check if the bot is mentioned
	isMentioned := false
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			isMentioned = true
			break
		}
	}

	if !isMentioned {
		return
	}

	// get the thread ID
	threadID := m.ChannelID

	// if message is not in a thread, reply with error
	if m.MessageReference == nil {
		s.ChannelMessageSend(m.ChannelID, "Mentioned the bot outside of a thread. Please send your message in a thread.")
		return
	}

	// try to lazy load session for this thread
	slog.Debug("lazy loading session", "thread_id", threadID)
	sessionData := lazyLoadSession(threadID)
	if sessionData == nil {
		s.ChannelMessageSend(m.ChannelID, "No OpenCode session found for this thread. Please start a session first using `/opencode` command.")
		return
	}

	// spawn session listener if not already active (atomic operation)
	spawnListenerIfNotExists(mainContext, mainWaitGroup, threadID)

	// remove bot mention from the message
	content := m.Content
	for _, mention := range m.Mentions {
		if mention.ID == s.State.User.ID {
			content = strings.ReplaceAll(content, fmt.Sprintf("<@%s>", mention.ID), "")
			content = strings.ReplaceAll(content, fmt.Sprintf("<@!%s>", mention.ID), "")
		}
	}
	content = strings.TrimSpace(content)

	if content == "" {
		s.ChannelMessageSend(m.ChannelID, "Please provide a message to send to OpenCode.")
		return
	}

	// send typing indicator
	s.ChannelTyping(m.ChannelID)

	// send message to opencode
	response := SendMessage(threadID, content)
	if response == nil {
		s.ChannelMessageSend(m.ChannelID, "Failed to send message to OpenCode.")
		return
	}
}
