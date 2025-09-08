package main

import (
	"regexp"
	"strings"
)

// formatBlockquote adds blockquote to text
func formatBlockquote(text string) string {
	text = strings.TrimRight(text, "\n")
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" { // skip blank lines
			out = append(out, "> "+line)
		}
	}
	return strings.Join(out, "\n")
}

func removeExcessiveNewLine(text string) string {
	text = strings.Trim(text, "\n")
	return regexp.MustCompile(`\n+`).ReplaceAllString(text, "\n")
}

// appendToContentHistory appends content with smart newline handling
func appendToContentHistory(existing, newContent string) string {
	if existing == "" {
		return newContent
	}
	if strings.HasSuffix(existing, "\n") {
		return existing + newContent
	}
	return existing + "\n" + newContent
}
