package sanitizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// SanitizeForModel removes content that the target model doesn't support.
// For DeepSeek, it strips <thinking>...</thinking> blocks from message content.
func SanitizeForModel(body []byte, target string) ([]byte, error) {
	if target == "deepseek" {
		return stripThinkingBlocks(body)
	}
	// For other models, pass through unchanged.
	return body, nil
}

// stripThinkingBlocks removes <thinking>...</thinking> tags from message content
// in a JSON request body. Works at the byte level to preserve all other structure.
func stripThinkingBlocks(body []byte) ([]byte, error) {
	if !bytes.Contains(body, []byte("<thinking>")) {
		return body, nil
	}

	// We need to operate on string content within JSON string values.
	// The safest approach: parse, walk, and rebuild.
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse body for sanitization: %w", err)
	}

	result := walkAndStrip(raw)
	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("rebuild body after sanitization: %w", err)
	}
	return out, nil
}

// walkAndStrip recursively walks a parsed JSON value and strips thinking blocks from strings.
func walkAndStrip(v any) any {
	switch x := v.(type) {
	case string:
		return stripFromString(x)
	case map[string]any:
		for k, val := range x {
			x[k] = walkAndStrip(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = walkAndStrip(val)
		}
		return x
	default:
		return v
	}
}

// stripFromString removes all <thinking>...</thinking> blocks from a string,
// trying to handle nested tags reasonably.
func stripFromString(s string) string {
	var result strings.Builder
	remaining := s

	for {
		start := strings.Index(remaining, "<thinking>")
		if start == -1 {
			result.WriteString(remaining)
			break
		}

		// Write everything before the opening tag.
		result.WriteString(remaining[:start])

		// Find the matching closing tag, handling nesting.
		depth := 1
		pos := start + len("<thinking>")
		for depth > 0 && pos < len(remaining) {
			nextOpen := strings.Index(remaining[pos:], "<thinking>")
			nextClose := strings.Index(remaining[pos:], "</thinking>")

			if nextClose == -1 {
				// No closing tag found — treat rest as thinking block and stop.
				pos = len(remaining)
				break
			}

			if nextOpen != -1 && nextOpen < nextClose {
				// Nested opening tag.
				depth++
				pos += nextOpen + len("<thinking>")
			} else {
				// Closing tag.
				depth--
				pos += nextClose + len("</thinking>")
			}
		}

		remaining = remaining[pos:]
	}

	return result.String()
}

// ContainsThinkingBlocks checks if the body contains any thinking blocks.
func ContainsThinkingBlocks(body []byte) bool {
	return bytes.Contains(body, []byte("<thinking>"))
}