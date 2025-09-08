package main

import (
	"testing"
)

func TestFormatBlockquote(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "Hello world",
			expected: "> Hello world",
		},
		{
			name:     "multiline text",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "> Line 1\n> Line 2\n> Line 3",
		},
		{
			name:     "text with trailing newlines",
			input:    "Hello\nworld\n\n",
			expected: "> Hello\n> world",
		},
		{
			name:     "text with blank lines",
			input:    "Line 1\n\nLine 3",
			expected: "> Line 1\n> Line 3",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only newlines",
			input:    "\n\n\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBlockquote(tt.input)
			if result != tt.expected {
				t.Errorf("formatBlockquote(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRemoveExcessiveNewLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "multiple consecutive newlines",
			input:    "Line 1\n\n\nLine 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "leading and trailing newlines",
			input:    "\n\nHello world\n\n",
			expected: "Hello world",
		},
		{
			name:     "many consecutive newlines",
			input:    "Start\n\n\n\n\nEnd",
			expected: "Start\nEnd",
		},
		{
			name:     "only newlines",
			input:    "\n\n\n\n",
			expected: "",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single newline",
			input:    "Hello\nworld",
			expected: "Hello\nworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeExcessiveNewLine(tt.input)
			if result != tt.expected {
				t.Errorf("removeExcessiveNewLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAppendToContentHistory(t *testing.T) {
	tests := []struct {
		name       string
		existing   string
		newContent string
		expected   string
	}{
		{
			name:       "empty existing content",
			existing:   "",
			newContent: "New content",
			expected:   "New content",
		},
		{
			name:       "existing content without trailing newline",
			existing:   "Existing content",
			newContent: "New content",
			expected:   "Existing content\nNew content",
		},
		{
			name:       "existing content with trailing newline",
			existing:   "Existing content\n",
			newContent: "New content",
			expected:   "Existing content\nNew content",
		},
		{
			name:       "both empty",
			existing:   "",
			newContent: "",
			expected:   "",
		},
		{
			name:       "empty new content",
			existing:   "Existing content",
			newContent: "",
			expected:   "Existing content\n",
		},
		{
			name:       "multiline existing content",
			existing:   "Line 1\nLine 2\n",
			newContent: "Line 3",
			expected:   "Line 1\nLine 2\nLine 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendToContentHistory(tt.existing, tt.newContent)
			if result != tt.expected {
				t.Errorf("appendToContentHistory(%q, %q) = %q, want %q", tt.existing, tt.newContent, result, tt.expected)
			}
		})
	}
}
