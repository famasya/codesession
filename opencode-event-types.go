package main

type MessagePartUpdated struct {
	Type       string `json:"type"`
	Properties struct {
		Part MessagePart `json:"part"`
	} `json:"properties"`
}

type MessagePart struct {
	ID        string `json:"id"`
	MessageID string `json:"messageID"`
	SessionID string `json:"sessionID"`
	Type      string `json:"type"`

	// Optional fields based on part type
	Text   string `json:"text,omitempty"`
	Tool   string `json:"tool,omitempty"`
	CallID string `json:"callID,omitempty"`

	// State field for tool parts
	State *ToolState `json:"state,omitempty"`

	// Time tracking
	Time *TimeRange `json:"time,omitempty"`

	// Tokens and cost info for step-finish parts
	Tokens *TokenInfo `json:"tokens,omitempty"`
	Cost   *float64   `json:"cost,omitempty"`
}

type ToolState struct {
	Status   string                 `json:"status"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Output   string                 `json:"output,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Time     *TimeRange             `json:"time,omitempty"`
}

type TimeRange struct {
	Start int64  `json:"start"`
	End   *int64 `json:"end,omitempty"`
}

type TokenInfo struct {
	Input     int       `json:"input"`
	Output    int       `json:"output"`
	Reasoning int       `json:"reasoning"`
	Cache     CacheInfo `json:"cache"`
}

type CacheInfo struct {
	Write int `json:"write"`
	Read  int `json:"read"`
}

// Part types identified from logs:
const (
	PartTypeText       = "text"        // User input text
	PartTypeReasoning  = "reasoning"   // AI reasoning/thinking
	PartTypeStepStart  = "step-start"  // Start of reasoning step
	PartTypeStepFinish = "step-finish" // End of reasoning step with tokens/cost
	PartTypeTool       = "tool"        // Tool execution (webfetch, etc.)
)

// Tool states observed:
const (
	ToolStatusPending   = "pending"
	ToolStatusRunning   = "running"
	ToolStatusCompleted = "completed"
)
