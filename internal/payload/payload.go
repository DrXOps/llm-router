package payload

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Message represents a single message in the Anthropic Messages API format.
// Content is kept as raw json.RawMessage to preserve structure without loss.
type Message struct {
	Role    string           `json:"role"`
	Content json.RawMessage `json:"content"`
}

// Payload represents an Anthropic /v1/messages request body.
// Only fields we inspect are typed; everything else is preserved as raw JSON.
type Payload struct {
	Model    string              `json:"model"`
	Messages []Message           `json:"messages"`
	Raw      map[string]any      `json:"-"`
}

// Parse decodes a JSON request body into a Payload, preserving unknown fields.
func Parse(body []byte) (*Payload, error) {
	var rawMap map[string]any
	if err := json.Unmarshal(body, &rawMap); err != nil {
		return nil, fmt.Errorf("parse request body: %w", err)
	}

	p := &Payload{
		Raw: make(map[string]any),
	}

	// Extract known fields.
	if model, ok := rawMap["model"].(string); ok {
		p.Model = model
	}
	delete(rawMap, "model")

	// Parse messages.
	if msgsRaw, ok := rawMap["messages"]; ok {
		msgsJSON, err := json.Marshal(msgsRaw)
		if err != nil {
			return nil, fmt.Errorf("re-marshal messages: %w", err)
		}
		var msgs []Message
		if err := json.Unmarshal(msgsJSON, &msgs); err != nil {
			return nil, fmt.Errorf("parse messages: %w", err)
		}
		p.Messages = msgs
		delete(rawMap, "messages")
	}

	// Everything else goes into Raw.
	for k, v := range rawMap {
		p.Raw[k] = v
	}

	return p, nil
}

// Rebuild serializes the payload back to JSON, replacing the model field.
// All original fields (system, tools, stream, etc.) are preserved via Raw.
func Rebuild(p *Payload, newModel string) ([]byte, error) {
	// Start from the raw fields (everything except model and messages).
	out := make(map[string]any)
	for k, v := range p.Raw {
		out[k] = v
	}

	// Set the new model.
	out["model"] = newModel

	// Set messages.
	out["messages"] = p.Messages

	return json.Marshal(out)
}

// ApproxTokens estimates the number of tokens in a text string.
// Simple heuristic: 1 token ≈ 4 characters for English text.
func ApproxTokens(text string) int {
	return ApproxTokensFromChars(len([]rune(text)))
}

// ApproxTokensFromChars estimates tokens from a character count.
func ApproxTokensFromChars(charCount int) int {
	if charCount == 0 {
		return 0
	}
	tokens := charCount / 4
	if tokens < 1 {
		tokens = 1
	}
	return tokens
}

// CountMessages returns the total number of messages.
func (p *Payload) CountMessages() int {
	return len(p.Messages)
}

// LastUserMessage returns the text content of the last message with role "user".
// Returns empty string if no user message is found.
func (p *Payload) LastUserMessage() string {
	for i := len(p.Messages) - 1; i >= 0; i-- {
		if p.Messages[i].Role == "user" {
			return extractText(p.Messages[i].Content)
		}
	}
	return ""
}

// AllUserMessages returns all user message texts.
func (p *Payload) AllUserMessages() []string {
	var msgs []string
	for _, m := range p.Messages {
		if m.Role == "user" {
			msgs = append(msgs, extractText(m.Content))
		}
	}
	return msgs
}

// TotalChars returns the total character count across all messages.
func (p *Payload) TotalChars() int {
	total := 0
	for _, m := range p.Messages {
		total += len(string(m.Content))
	}
	return total
}

// HasToolCalls returns true if any message content contains tool_use or tool_result.
func (p *Payload) HasToolCalls() bool {
	for _, m := range p.Messages {
		content := string(m.Content)
		if strings.Contains(content, "tool_use") || strings.Contains(content, "tool_result") {
			return true
		}
	}
	return false
}

// extractText extracts plain text from a raw message content.
// Content can be a string or an array of content blocks.
func extractText(raw json.RawMessage) string {
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}

	// Try array of content blocks with "text" key.
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var texts []string
		for _, block := range blocks {
			if block["type"] == "text" {
				if t, ok := block["text"].(string); ok {
					texts = append(texts, t)
				}
			}
		}
		return strings.Join(texts, "\n")
	}

	return ""
}