package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMessagePartSerialization(t *testing.T) {
	part := MessagePart{
		ID:        "test-id",
		MessageID: "msg-123",
		SessionID: "session-456",
		Type:      PartTypeText,
		Text:      "Hello world",
		Tool:      "webfetch",
		CallID:    "call-789",
		State: &ToolState{
			Status: ToolStatusCompleted,
			Input:  map[string]interface{}{"url": "https://example.com"},
			Output: "Success",
			Title:  "Web Fetch",
		},
		Time: &TimeRange{
			Start: 1234567890,
			End:   &[]int64{1234567900}[0],
		},
		Tokens: &TokenInfo{
			Input:     100,
			Output:    50,
			Reasoning: 25,
			Cache: CacheInfo{
				Write: 10,
				Read:  5,
			},
		},
		Cost: &[]float64{0.001}[0],
	}

	// Test JSON marshaling
	data, err := json.Marshal(part)
	if err != nil {
		t.Fatalf("Failed to marshal MessagePart: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MessagePart
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal MessagePart: %v", err)
	}

	// Verify all fields
	if unmarshaled.ID != part.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, part.ID)
	}
	if unmarshaled.Type != part.Type {
		t.Errorf("Type mismatch: got %s, want %s", unmarshaled.Type, part.Type)
	}
	if unmarshaled.Text != part.Text {
		t.Errorf("Text mismatch: got %s, want %s", unmarshaled.Text, part.Text)
	}
	if unmarshaled.State.Status != part.State.Status {
		t.Errorf("State.Status mismatch: got %s, want %s", unmarshaled.State.Status, part.State.Status)
	}
}

func TestCommitRecordSerialization(t *testing.T) {
	timestamp := time.Now()
	commit := CommitRecord{
		Hash:      "abc123",
		Summary:   "Initial commit",
		Timestamp: timestamp,
		Status:    "success",
	}

	// Test JSON marshaling
	data, err := json.Marshal(commit)
	if err != nil {
		t.Fatalf("Failed to marshal CommitRecord: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled CommitRecord
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal CommitRecord: %v", err)
	}

	// Verify all fields
	if unmarshaled.Hash != commit.Hash {
		t.Errorf("Hash mismatch: got %s, want %s", unmarshaled.Hash, commit.Hash)
	}
	if unmarshaled.Summary != commit.Summary {
		t.Errorf("Summary mismatch: got %s, want %s", unmarshaled.Summary, commit.Summary)
	}
	if unmarshaled.Status != commit.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshaled.Status, commit.Status)
	}
	// Note: Time comparison might need tolerance for precision differences
	if !unmarshaled.Timestamp.Equal(commit.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", unmarshaled.Timestamp, commit.Timestamp)
	}
}

func TestSessionDataSerialization(t *testing.T) {
	sessionData := SessionData{
		ThreadID:       "thread-123",
		SessionID:      "session-456",
		Model:          Model{ProviderID: "openai", ModelID: "gpt-4"},
		WorktreePath:   "/tmp/worktree",
		RepositoryPath: "/repo/path",
		RepositoryName: "test-repo",
		CreatedAt:      time.Now(),
		Commits: []CommitRecord{
			{
				Hash:      "commit1",
				Summary:   "First commit",
				Timestamp: time.Now(),
				Status:    "success",
			},
		},
		// Non-serialized fields should be ignored
		Active:      true,
		IsStreaming: true,
		UserID:      "user-123",
	}

	// Test JSON marshaling
	data, err := json.Marshal(sessionData)
	if err != nil {
		t.Fatalf("Failed to marshal SessionData: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled SessionData
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SessionData: %v", err)
	}

	// Verify serialized fields
	if unmarshaled.ThreadID != sessionData.ThreadID {
		t.Errorf("ThreadID mismatch: got %s, want %s", unmarshaled.ThreadID, sessionData.ThreadID)
	}
	if unmarshaled.SessionID != sessionData.SessionID {
		t.Errorf("SessionID mismatch: got %s, want %s", unmarshaled.SessionID, sessionData.SessionID)
	}
	if len(unmarshaled.Commits) != len(sessionData.Commits) {
		t.Errorf("Commits length mismatch: got %d, want %d", len(unmarshaled.Commits), len(sessionData.Commits))
	}

	// Verify non-serialized fields are zero values
	if unmarshaled.Active != false {
		t.Error("Active should be false after unmarshaling (not serialized)")
	}
	if unmarshaled.IsStreaming != false {
		t.Error("IsStreaming should be false after unmarshaling (not serialized)")
	}
	if unmarshaled.UserID != "" {
		t.Error("UserID should be empty after unmarshaling (not serialized)")
	}
}

func TestToolStateValidation(t *testing.T) {
	tests := []struct {
		name   string
		status string
		valid  bool
	}{
		{"valid pending", ToolStatusPending, true},
		{"valid running", ToolStatusRunning, true},
		{"valid completed", ToolStatusCompleted, true},
		{"invalid status", "invalid", false},
		{"empty status", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := ToolState{Status: tt.status}

			// For now, we just check the constants exist and can be used
			switch state.Status {
			case ToolStatusPending, ToolStatusRunning, ToolStatusCompleted:
				if !tt.valid {
					t.Errorf("Status %s should be invalid", tt.status)
				}
			default:
				if tt.valid {
					t.Errorf("Status %s should be valid", tt.status)
				}
			}
		})
	}
}

func TestPartTypeConstants(t *testing.T) {
	expectedTypes := []string{
		PartTypeText,
		PartTypeReasoning,
		PartTypeStepStart,
		PartTypeStepFinish,
		PartTypeTool,
	}

	expectedValues := []string{
		"text",
		"reasoning",
		"step-start",
		"step-finish",
		"tool",
	}

	if len(expectedTypes) != len(expectedValues) {
		t.Fatal("Mismatch between expected types and values")
	}

	for i, expectedType := range expectedTypes {
		if expectedType != expectedValues[i] {
			t.Errorf("Part type constant mismatch: got %s, want %s", expectedType, expectedValues[i])
		}
	}
}

func TestTimeRangeWithNilEnd(t *testing.T) {
	timeRange := TimeRange{
		Start: 1234567890,
		End:   nil,
	}

	data, err := json.Marshal(timeRange)
	if err != nil {
		t.Fatalf("Failed to marshal TimeRange with nil End: %v", err)
	}

	var unmarshaled TimeRange
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal TimeRange: %v", err)
	}

	if unmarshaled.Start != timeRange.Start {
		t.Errorf("Start mismatch: got %d, want %d", unmarshaled.Start, timeRange.Start)
	}
	if unmarshaled.End != nil {
		t.Errorf("End should be nil after unmarshaling, got %v", unmarshaled.End)
	}
}
