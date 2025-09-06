package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/goombaio/namegenerator"
	"github.com/sst/opencode-sdk-go"
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

	if command == "commit" {
		handleCommitCommand(s, i)
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
	slog.Debug("creating thread", "thread_name", threadName, "channel_id", i.ChannelID)
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
	slog.Debug("thread created successfully", "thread_id", thread.ID, "thread_name", thread.Name)

	// Create worktree directory in current repository
	currentDir, _ := os.Getwd()
	worktreeDir := filepath.Join(currentDir, ".worktrees", thread.ID)
	err = os.MkdirAll(filepath.Dir(worktreeDir), 0755)
	if err != nil {
		slog.Error("failed to create worktrees directory", "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create worktrees directory"}[0],
		})
		return
	}

	// Create git worktree FIRST
	cmd := exec.Command("git", "worktree", "add", worktreeDir)
	cmd.Dir = currentDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to create git worktree", "error", err, "output", string(output))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create git worktree"}[0],
		})
		return
	}

	// Create session AFTER worktree is created
	slog.Debug("creating session", "thread_id", thread.ID, "worktree_dir", worktreeDir)
	session := GetOrCreateSession(thread.ID, worktreeDir, repository.Path, repository.Name)
	if session == nil {
		slog.Error("failed to create session", "thread_id", thread.ID)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to create session"}[0],
		})
		return
	}
	slog.Debug("session created successfully", "thread_id", thread.ID, "session_id", session.ID)

	// Set the selected model in session data
	slog.Debug("setting model in session data", "thread_id", thread.ID)
	sessionMutex.Lock()
	slog.Debug("acquired session mutex", "thread_id", thread.ID)
	if sessionData, exists := sessionCache[thread.ID]; exists {
		slog.Debug("found session in cache", "thread_id", thread.ID)
		sessionData.Model = model

		// Save session data without acquiring mutex again (we already hold it)
		data, err := json.MarshalIndent(sessionData, "", "  ")
		if err != nil {
			slog.Error("failed to marshal session data", "error", err)
		} else {
			filePath := filepath.Join(sessionsDirectory, fmt.Sprintf("%s.json", sessionData.ThreadID))
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				slog.Error("failed to save session data with model", "error", err)
			} else {
				slog.Debug("saved session data with model", "thread_id", thread.ID)
			}
		}
	} else {
		slog.Error("session not found in cache", "thread_id", thread.ID)
	}
	sessionMutex.Unlock()
	slog.Debug("released session mutex", "thread_id", thread.ID)

	// Send initial message to the thread
	slog.Debug("sending welcome message to thread", "thread_id", thread.ID)
	trimmedWorktreeDir := strings.TrimPrefix(worktreeDir, repository.Path)
	trimmedSessionID := session.ID[len(session.ID)-8:]
	welcomeMessage := fmt.Sprintf("```\nOpenCode Session Started\nRepository: %s\nModel: %s\nWorktree Path: `%s`\nSession ID: %s\n```",
		repository.Name, model.ProviderID+"/"+model.ModelID, trimmedWorktreeDir, trimmedSessionID)

	_, err = s.ChannelMessageSend(thread.ID, welcomeMessage)
	if err != nil {
		slog.Error("failed to send welcome message", "thread_id", thread.ID, "error", err)
	} else {
		slog.Debug("welcome message sent successfully", "thread_id", thread.ID)
	}

	// Update the interaction response with success message AFTER welcome message
	slog.Debug("updating interaction response", "thread_id", thread.ID)
	s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &[]string{fmt.Sprintf("OpenCode session created successfully! Check the thread: %s", thread.Mention())}[0],
	})
}

// constructPRLink parses the remote URL and constructs a PR link for the given branch
func constructPRLink(remoteURL, branch string) string {
	// Remove .git suffix if present
	remoteURL = strings.TrimSuffix(remoteURL, ".git")

	// Handle different URL formats
	var repoPath string
	if strings.HasPrefix(remoteURL, "https://") {
		// HTTPS format: https://github.com/user/repo
		if strings.Contains(remoteURL, "github.com") {
			repoPath = strings.TrimPrefix(remoteURL, "https://github.com/")
			return fmt.Sprintf("https://github.com/%s/compare/%s?expand=1", repoPath, branch)
		} else if strings.Contains(remoteURL, "gitlab.com") {
			repoPath = strings.TrimPrefix(remoteURL, "https://gitlab.com/")
			return fmt.Sprintf("https://gitlab.com/%s/-/merge_requests/new?merge_request[source_branch]=%s", repoPath, branch)
		}
	} else if strings.HasPrefix(remoteURL, "git@") {
		// SSH format: git@github.com:user/repo.git
		if strings.Contains(remoteURL, "github.com") {
			repoPath = strings.TrimPrefix(remoteURL, "git@github.com:")
			repoPath = strings.Replace(repoPath, ":", "/", 1)
			return fmt.Sprintf("https://github.com/%s/compare/%s?expand=1", repoPath, branch)
		} else if strings.Contains(remoteURL, "gitlab.com") {
			repoPath = strings.TrimPrefix(remoteURL, "git@gitlab.com:")
			repoPath = strings.Replace(repoPath, ":", "/", 1)
			return fmt.Sprintf("https://gitlab.com/%s/-/merge_requests/new?merge_request[source_branch]=%s", repoPath, branch)
		}
	}

	// If we can't determine the hosting service, return empty string
	return ""
}

func handleCommitCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channelID := i.ChannelID
	slog.Debug("starting commit command", "channel_id", channelID)

	// Get channel info to check if it's a thread
	channel, err := s.Channel(channelID)
	if err != nil {
		slog.Error("failed to get channel info", "channel_id", channelID, "error", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Failed to get channel information.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Check if command is executed in a thread
	if channel.Type != discordgo.ChannelTypeGuildPublicThread && channel.Type != discordgo.ChannelTypeGuildPrivateThread {
		slog.Debug("commit command executed outside thread", "channel_id", channelID, "channel_type", channel.Type)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "The `/commit` command can only be used within a codesession thread. Please use this command in a thread created by the `/opencode` command.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	threadID := channelID
	slog.Debug("commit command in thread", "thread_id", threadID)

	// Defer response
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
	if err != nil {
		slog.Error("failed to defer commit interaction", "thread_id", threadID, "error", err)
		return
	}
	slog.Debug("commit interaction deferred successfully", "thread_id", threadID)

	// Check if session exists
	slog.Debug("attempting to load session", "thread_id", threadID)
	session := lazyLoadSession(threadID)
	if session == nil {
		slog.Error("no session found for thread", "thread_id", threadID)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"No OpenCode session found for this thread. Please start a session first using `/opencode` command."}[0],
		})
		return
	}
	slog.Debug("session loaded successfully", "thread_id", threadID, "session_id", session.SessionID)

	// Use the stored worktree path from session data
	worktreePath := session.WorktreePath
	slog.Debug("using stored worktree path", "thread_id", threadID, "worktree_path", worktreePath, "repository_path", session.RepositoryPath, "repository_name", session.RepositoryName)

	// Validate worktree directory exists
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		slog.Error("worktree directory does not exist", "thread_id", threadID, "worktree_path", worktreePath)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Worktree directory not found. Please start a new session."}[0],
		})
		return
	}
	slog.Debug("worktree directory exists", "thread_id", threadID, "worktree_path", worktreePath)

	// send message to opencode to generate commit summary
	slog.Debug("requesting AI summary for commit", "thread_id", threadID, "session_id", session.SessionID)
	instruction := AppConfig.SummarizerInstruction
	if instruction == "" {
		instruction = "Generate a git commit message in conventional commit format. The first line should be in the format 'type(scope): description'. Follow with a bullet-point list of key changes made in the session. Keep the entire message concise."
	}
	client := Opencode()
	response, err := client.Session.Prompt(context.Background(), session.SessionID, opencode.SessionPromptParams{
		Directory: opencode.F(worktreePath),
		Tools: opencode.F(map[string]bool{
			"write": false,
			"edit":  false,
		}),
		Parts: opencode.F([]opencode.SessionPromptParamsPartUnion{
			&opencode.TextPartInputParam{
				Type: opencode.F(opencode.TextPartInputTypeText),
				Text: opencode.F(instruction),
			},
		}),
		Model: opencode.F(opencode.SessionPromptParamsModel{
			ProviderID: opencode.F(session.Model.ProviderID),
			ModelID:    opencode.F(session.Model.ModelID),
		}),
	})
	if err != nil {
		slog.Error("failed to generate AI summary", "thread_id", threadID, "error", err)
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to generate summary."}[0],
		})
		return
	}
	slog.Debug("AI summary generated successfully", "thread_id", threadID, "parts_count", len(response.Parts))

	// Get summary from response by looking specifically for "text" type parts
	summary := ""
	for i, part := range response.Parts {
		slog.Debug("checking response part", "thread_id", threadID, "part_index", i, "part_type", part.Type, "text_length", len(part.Text))
		if part.Type == "text" && part.Text != "" {
			summary = part.Text
			slog.Debug("found AI summary in text part", "thread_id", threadID, "part_index", i, "raw_summary", summary, "length", len(summary))
			if len(summary) > 50 {
				summary = summary[:50]
				slog.Debug("summary truncated to 50 chars", "thread_id", threadID, "truncated_summary", summary)
			}
			break // Use the first text-type part we find
		}
	}
	if summary == "" {
		summary = "Changes made during session"
		slog.Debug("using default summary", "thread_id", threadID, "summary", summary)
	} else {
		slog.Debug("final summary prepared", "thread_id", threadID, "summary", summary)
	}

	// Create a pending commit record
	commitRecord := CommitRecord{
		Summary:   summary,
		Timestamp: time.Now(),
		Status:    "pending",
	}

	// Add pending commit to session
	sessionMutex.Lock()
	session.Commits = append(session.Commits, commitRecord)
	sessionMutex.Unlock()
	slog.Debug("added pending commit record", "thread_id", threadID, "summary", summary)

	// Check git status before adding
	slog.Debug("checking git status before staging", "thread_id", threadID)
	statusCmd := exec.Command("git", "status", "--porcelain")
	statusCmd.Dir = worktreePath
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to check git status", "thread_id", threadID, "error", err, "output", string(statusOutput))
	} else {
		slog.Debug("git status output", "thread_id", threadID, "status", string(statusOutput))
		if len(strings.TrimSpace(string(statusOutput))) == 0 {
			slog.Debug("no changes detected in worktree", "thread_id", threadID)

			// Update commit record with "no changes" status
			sessionMutex.Lock()
			if len(session.Commits) > 0 {
				session.Commits[len(session.Commits)-1].Status = "no_changes"
			}
			sessionMutex.Unlock()

			// Save session data after releasing mutex to avoid deadlock
			if err := saveSessionData(session); err != nil {
				slog.Error("failed to save session data for no changes", "thread_id", threadID, "error", err)
			}

			s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
				Content: &[]string{"No changes to commit."}[0],
			})
			return
		}
	}

	// Git add operation
	slog.Debug("staging all changes", "thread_id", threadID)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	addOutput, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to git add", "thread_id", threadID, "error", err, "output", string(addOutput))
		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to stage changes."}[0],
		})
		return
	}
	slog.Debug("git add completed successfully", "thread_id", threadID, "output", string(addOutput))

	// Git commit operation
	slog.Debug("committing changes", "thread_id", threadID, "commit_message", summary)
	cmd = exec.Command("git", "commit", "-m", summary)
	cmd.Dir = worktreePath
	commitOutput, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to git commit", "thread_id", threadID, "error", err, "output", string(commitOutput))

		// Update commit record with failed status
		sessionMutex.Lock()
		if len(session.Commits) > 0 {
			session.Commits[len(session.Commits)-1].Status = "failed"
		}
		sessionMutex.Unlock()

		// Save session data after releasing mutex to avoid deadlock
		if err := saveSessionData(session); err != nil {
			slog.Error("failed to save session data for commit failure", "thread_id", threadID, "error", err)
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to commit changes."}[0],
		})
		return
	}
	slog.Debug("git commit completed successfully", "thread_id", threadID, "output", string(commitOutput))

	// Get commit hash for logging
	var commitHash string
	hashCmd := exec.Command("git", "rev-parse", "HEAD")
	hashCmd.Dir = worktreePath
	if hashOutput, err := hashCmd.CombinedOutput(); err == nil {
		commitHash = strings.TrimSpace(string(hashOutput))
		slog.Debug("commit hash", "thread_id", threadID, "commit_hash", commitHash)
	}

	// Check current branch before push
	var currentBranch string
	branchCmd := exec.Command("git", "branch", "--show-current")
	branchCmd.Dir = worktreePath
	if branchOutput, err := branchCmd.CombinedOutput(); err == nil {
		currentBranch = strings.TrimSpace(string(branchOutput))
		slog.Debug("current branch", "thread_id", threadID, "branch", currentBranch)
	} else {
		slog.Error("failed to get current branch", "thread_id", threadID, "error", err)
		currentBranch = "main" // fallback to main branch
	}

	// Git push operation with specific branch
	slog.Debug("pushing changes to remote", "thread_id", threadID, "branch", currentBranch)
	cmd = exec.Command("git", "push", "origin", currentBranch)
	cmd.Dir = worktreePath
	pushOutput, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("failed to git push", "thread_id", threadID, "error", err, "output", string(pushOutput))

		// Update commit record with failed status (commit succeeded but push failed)
		sessionMutex.Lock()
		if len(session.Commits) > 0 {
			session.Commits[len(session.Commits)-1].Status = "failed"
			session.Commits[len(session.Commits)-1].Hash = commitHash
		}
		sessionMutex.Unlock()

		// Save session data after releasing mutex to avoid deadlock
		if err := saveSessionData(session); err != nil {
			slog.Error("failed to save session data for push failure", "thread_id", threadID, "error", err)
		}

		s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
			Content: &[]string{"Failed to push changes."}[0],
		})
		return
	}
	slog.Debug("git push completed successfully", "thread_id", threadID, "output", string(pushOutput))

	// Update commit record with success status
	sessionMutex.Lock()
	if len(session.Commits) > 0 {
		session.Commits[len(session.Commits)-1].Status = "success"
		session.Commits[len(session.Commits)-1].Hash = commitHash
		slog.Debug("updating commit record with success status", "thread_id", threadID, "commit_hash", commitHash)
	}
	sessionMutex.Unlock()

	// Save session data after releasing the mutex to avoid deadlock
	slog.Debug("about to save session data", "thread_id", threadID)
	if err := saveSessionData(session); err != nil {
		slog.Error("failed to save session data after successful commit", "thread_id", threadID, "error", err)
	} else {
		slog.Debug("saved session data with success status", "thread_id", threadID, "commit_hash", commitHash)
	}

	// Get PR/MR link if available
	var prLink string
	remoteCmd := exec.Command("git", "remote", "get-url", "origin")
	remoteCmd.Dir = worktreePath
	if remoteOutput, err := remoteCmd.CombinedOutput(); err == nil {
		remoteURL := strings.TrimSpace(string(remoteOutput))
		slog.Debug("got remote URL", "thread_id", threadID, "remote_url", remoteURL)

		// Parse remote URL to extract repository info and construct PR link
		if prURL := constructPRLink(remoteURL, currentBranch); prURL != "" {
			prLink = prURL
			slog.Debug("constructed PR link", "thread_id", threadID, "pr_link", prLink)
		}
	} else {
		slog.Debug("failed to get remote URL", "thread_id", threadID, "error", err)
	}

	// Send detailed success message to thread with git push output
	slog.Debug("preparing detailed success message", "thread_id", threadID)
	slog.Debug("sending detailed success message to thread", "thread_id", threadID)

	detailedMessage := fmt.Sprintf("**Commit & Push Successful**\n\n**Summary:** %s\n**Hash:** %s\n**Branch:** %s",
		summary, commitHash, currentBranch)

	if prLink != "" {
		detailedMessage += fmt.Sprintf("\n\n**Pull Request:** %s", prLink)
	}

	detailedMessage += fmt.Sprintf("\n\n**Git Push Output:**\n```\n%s\n```", strings.TrimSpace(string(pushOutput)))

	_, err = s.ChannelMessageSend(threadID, detailedMessage)
	if err != nil {
		slog.Error("failed to send detailed success message", "thread_id", threadID, "error", err)
	} else {
		slog.Debug("detailed success message sent to thread", "thread_id", threadID)
	}

	// Update interaction response
	slog.Debug("updating interaction response with success", "thread_id", threadID)
	_, interactionErr := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &[]string{"Commit completed successfully!"}[0],
	})
	if interactionErr != nil {
		slog.Error("failed to update interaction response", "thread_id", threadID, "error", interactionErr)
	} else {
		slog.Debug("interaction response updated successfully", "thread_id", threadID)
	}

	slog.Debug("commit command completed successfully", "thread_id", threadID, "final_summary", summary, "commit_hash", commitHash)
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

	// get the channel info to check if it's a thread
	channel, err := s.Channel(m.ChannelID)
	if err != nil {
		slog.Error("failed to get channel info", "channel_id", m.ChannelID, "error", err)
		s.ChannelMessageSend(m.ChannelID, "Failed to get channel information.")
		return
	}

	// check if message is in a thread
	if channel.Type != discordgo.ChannelTypeGuildPublicThread && channel.Type != discordgo.ChannelTypeGuildPrivateThread {
		s.ChannelMessageSend(m.ChannelID, "Mentioned the bot outside of a thread. Please send your message in a thread.")
		return
	}

	threadID := m.ChannelID

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
