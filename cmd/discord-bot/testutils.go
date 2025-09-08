package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

// TestConfig provides a default configuration for testing
var TestConfig = Config{
	LogLevel:     "error", // Suppress logs during testing
	BotToken:     "test-discord-token",
	OpencodePort: 8080,
	Repositories: []Repository{
		{
			Name: "test-repo",
			Path: "/tmp/test-repo",
		},
	},
	Models: []Model{
		{
			ProviderID: "openai",
			ModelID:    "gpt-4",
		},
	},
}

// SetupTestConfig sets up a temporary configuration for testing
func SetupTestConfig(t *testing.T) func() {
	originalConfig := AppConfig
	AppConfig = TestConfig

	return func() {
		AppConfig = originalConfig
	}
}

// CreateTestSession creates a test session with default values
func CreateTestSession(t *testing.T, threadID string) *SessionData {
	return &SessionData{
		ThreadID:       threadID,
		SessionID:      fmt.Sprintf("session-%s", threadID),
		Model:          TestConfig.Models[0],
		WorktreePath:   filepath.Join("/tmp/worktrees", threadID),
		RepositoryPath: TestConfig.Repositories[0].Path,
		RepositoryName: TestConfig.Repositories[0].Name,
		CreatedAt:      time.Now(),
		Commits:        []CommitRecord{},
		Active:         false,
		IsStreaming:    false,
	}
}

// CreateTestSessionFile creates a test session file in the given directory
func CreateTestSessionFile(t *testing.T, sessionDir, threadID string, sessionData *SessionData) {
	sessionFile := filepath.Join(sessionDir, threadID+".json")
	data, err := json.Marshal(sessionData)
	if err != nil {
		t.Fatalf("Failed to marshal test session data: %v", err)
	}

	err = os.WriteFile(sessionFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test session file: %v", err)
	}
}

// MockDiscordSession provides a mock Discord session for testing
type MockDiscordSession struct {
	// Track method calls
	InteractionRespondCalled      bool
	InteractionResponseEditCalled bool
	ThreadStartCalled             bool
	ChannelMessageSendCalled      bool
	ChannelMessageEditCalled      bool

	// Mock responses
	ThreadStartResponse     *discordgo.Channel
	InteractionRespondError error
	ThreadStartError        error
	ChannelMessageSendError error

	// Store parameters for verification
	LastInteractionResponse  *discordgo.InteractionResponse
	LastThreadStartChannelID string
	LastThreadStartName      string
	LastMessageContent       string
}

func (m *MockDiscordSession) InteractionRespond(i *discordgo.Interaction, resp *discordgo.InteractionResponse) error {
	m.InteractionRespondCalled = true
	m.LastInteractionResponse = resp
	return m.InteractionRespondError
}

func (m *MockDiscordSession) InteractionResponseEdit(i *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
	m.InteractionResponseEditCalled = true
	return nil, nil
}

func (m *MockDiscordSession) ThreadStart(channelID, name string, typ discordgo.ChannelType, archiveDuration int) (*discordgo.Channel, error) {
	m.ThreadStartCalled = true
	m.LastThreadStartChannelID = channelID
	m.LastThreadStartName = name
	if m.ThreadStartResponse != nil {
		return m.ThreadStartResponse, m.ThreadStartError
	}
	return &discordgo.Channel{
		ID:   "test-thread-id",
		Name: name,
		Type: typ,
	}, m.ThreadStartError
}

func (m *MockDiscordSession) ChannelMessageSend(channelID string, content string) (*discordgo.Message, error) {
	m.ChannelMessageSendCalled = true
	m.LastMessageContent = content
	return &discordgo.Message{
		ID:      "test-message-id",
		Content: content,
	}, m.ChannelMessageSendError
}

func (m *MockDiscordSession) ChannelMessageEdit(channelID, messageID, content string) (*discordgo.Message, error) {
	m.ChannelMessageEditCalled = true
	m.LastMessageContent = content
	return &discordgo.Message{
		ID:      messageID,
		Content: content,
	}, nil
}

// MockOpenCodeSession provides a mock OpenCode session for testing
type MockOpenCodeSession struct {
	// Track method calls
	SendMessageCalled bool
	EventsCalled      bool

	// Mock responses
	SendMessageError error
	EventsChannel    chan MockEvent

	// Store parameters
	LastMessage string
}

type MockEvent struct {
	Type string
	Data interface{}
}

func (m *MockOpenCodeSession) SendMessage(ctx context.Context, message string) error {
	m.SendMessageCalled = true
	m.LastMessage = message
	return m.SendMessageError
}

func (m *MockOpenCodeSession) Events(ctx context.Context) <-chan interface{} {
	m.EventsCalled = true
	if m.EventsChannel == nil {
		m.EventsChannel = make(chan MockEvent, 10)
	}

	// Convert to interface{} channel
	eventChan := make(chan interface{}, 10)
	go func() {
		defer close(eventChan)
		for event := range m.EventsChannel {
			eventChan <- event
		}
	}()

	return eventChan
}

// TestInteractionCreate creates a mock Discord interaction for testing
func CreateTestInteraction(t *testing.T, commandName string, options ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			Type:      discordgo.InteractionApplicationCommand,
			ChannelID: "test-channel-id",
			GuildID:   "test-guild-id",
			User: &discordgo.User{
				ID:       "test-user-id",
				Username: "testuser",
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    commandName,
				Options: options,
			},
		},
	}
}

// CreateTestApplicationCommandOption creates a mock command option
func CreateTestApplicationCommandOption(name string, optionType discordgo.ApplicationCommandOptionType, value interface{}) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  optionType,
		Value: value,
	}
}

// CleanupTestSession cleans up test session data
func CleanupTestSession(t *testing.T, threadID string) {
	sessionMutex.Lock()
	delete(sessionCache, threadID)
	sessionMutex.Unlock()

	listenersMutex.Lock()
	if cancel, exists := activeListeners[threadID]; exists {
		cancel()
		delete(activeListeners, threadID)
	}
	listenersMutex.Unlock()
}

// AssertSessionInCache checks if a session exists in the cache
func AssertSessionInCache(t *testing.T, threadID string) *SessionData {
	sessionMutex.RLock()
	sessionData, exists := sessionCache[threadID]
	sessionMutex.RUnlock()

	if !exists {
		t.Errorf("Expected session %s to be in cache", threadID)
		return nil
	}

	return sessionData
}

// AssertSessionNotInCache checks if a session does not exist in the cache
func AssertSessionNotInCache(t *testing.T, threadID string) {
	sessionMutex.RLock()
	_, exists := sessionCache[threadID]
	sessionMutex.RUnlock()

	if exists {
		t.Errorf("Expected session %s not to be in cache", threadID)
	}
}
