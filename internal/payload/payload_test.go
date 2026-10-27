package payload

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseSimple(t *testing.T) {
	body := `{"model":"claude-haiku-5-5","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if p.Model != "claude-haiku-5-5" {
		t.Errorf("expected model, got %q", p.Model)
	}
	if len(p.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(p.Messages))
	}
	// Raw should preserve max_tokens.
	if _, ok := p.Raw["max_tokens"]; !ok {
		t.Error("Raw missing max_tokens")
	}
	// model and messages should be removed from Raw.
	if _, ok := p.Raw["model"]; ok {
		t.Error("Raw should not contain model")
	}
	if _, ok := p.Raw["messages"]; ok {
		t.Error("Raw should not contain messages")
	}
}

func TestParseMinimal(t *testing.T) {
	body := `{"messages":[]}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if p.Model != "" {
		t.Errorf("expected empty model, got %q", p.Model)
	}
	if len(p.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(p.Messages))
	}
}

func TestParseInvalidJSON(t *testing.T) {
	body := `{invalid}`
	_, err := Parse([]byte(body))
	if err == nil {
		t.Error("Parse() should fail with invalid JSON")
	}
}

func TestRebuildRoundtrip(t *testing.T) {
	body := `{"model":"claude-haiku-5-5","messages":[{"role":"user","content":"Hello"}],"max_tokens":100,"system":"You are a helpful assistant","stream":true,"temperature":0.7}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	rebuilt, err := Rebuild(p, "deepseek-v4-pro")
	if err != nil {
		t.Fatalf("Rebuild() failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(rebuilt, &result); err != nil {
		t.Fatalf("Rebuild produced invalid JSON: %v", err)
	}

	// Model should be replaced.
	if result["model"] != "deepseek-v4-pro" {
		t.Errorf("expected model deepseek-v4-pro, got %v", result["model"])
	}

	// Original model should NOT be present.
	if model, ok := result["original_model"]; ok {
		t.Errorf("unexpected key original_model: %v", model)
	}

	// Other fields should be preserved.
	if result["max_tokens"] != float64(100) {
		t.Errorf("expected max_tokens=100, got %v", result["max_tokens"])
	}
	if result["system"] != "You are a helpful assistant" {
		t.Errorf("expected system message, got %v", result["system"])
	}
	if result["stream"] != true {
		t.Errorf("expected stream=true, got %v", result["stream"])
	}
	if result["temperature"] != 0.7 {
		t.Errorf("expected temperature=0.7, got %v", result["temperature"])
	}

	// Messages should be present.
	msgs, ok := result["messages"].([]any)
	if !ok || len(msgs) != 1 {
		t.Errorf("expected messages array with 1 item, got %v", result["messages"])
	}
}

func TestRebuildWithEmptyModel(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Hi"}]}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	rebuilt, err := Rebuild(p, "haiku")
	if err != nil {
		t.Fatalf("Rebuild() failed: %v", err)
	}

	var result map[string]any
	json.Unmarshal(rebuilt, &result)
	if result["model"] != "haiku" {
		t.Errorf("expected model haiku, got %v", result["model"])
	}
}

func TestRebuildPreservesAllMessages(t *testing.T) {
	body := `{"model":"claude","messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"Hi there"},{"role":"user","content":"How are you?"}]}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}

	rebuilt, err := Rebuild(p, "deepseek")
	if err != nil {
		t.Fatalf("Rebuild() failed: %v", err)
	}

	var result map[string]any
	json.Unmarshal(rebuilt, &result)
	msgs := result["messages"].([]any)
	if len(msgs) != 3 {
		t.Errorf("expected 3 messages, got %d", len(msgs))
	}
}

func TestApproxTokens(t *testing.T) {
	cases := []struct {
		input  string
		expect int
	}{
		{"", 0},
		{"a", 1},
		{"hello", 1},
		{"Hello world, this is a test!", 6}, // 29 chars / 4 = 7.25 -> 7... actually 29/4 = 7
		{"abcdefgh", 2},
	}

	for _, c := range cases {
		got := ApproxTokens(c.input)
		// Allow some tolerance since this is a heuristic
		if got < c.expect/2 || got > c.expect*2+1 {
			t.Errorf("ApproxTokens(%q) = %d, expected around %d", c.input, got, c.expect)
		}
	}
}

func TestHasToolCalls(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":[{"type":"tool_use","id":"x","name":"test"}]}]}`
	p, _ := Parse([]byte(body))
	if !p.HasToolCalls() {
		t.Error("HasToolCalls() should be true")
	}
}

func TestHasToolCallsFalse(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Hello"}]}`
	p, _ := Parse([]byte(body))
	if p.HasToolCalls() {
		t.Error("HasToolCalls() should be false")
	}
}

func TestHasToolCallsResult(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":[{"type":"tool_result","tool_use_id":"x"}]}]}`
	p, _ := Parse([]byte(body))
	if !p.HasToolCalls() {
		t.Error("HasToolCalls() should detect tool_result")
	}
}

func TestLastUserMessage(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"First"},{"role":"assistant","content":"Reply"},{"role":"user","content":"Last message"}]}`
	p, _ := Parse([]byte(body))
	last := p.LastUserMessage()
	if !strings.Contains(last, "Last") {
		t.Errorf("expected last user message, got %q", last)
	}
}

func TestLastUserMessageEmpty(t *testing.T) {
	body := `{"messages":[]}`
	p, _ := Parse([]byte(body))
	last := p.LastUserMessage()
	if last != "" {
		t.Errorf("expected empty, got %q", last)
	}
}

func TestAllUserMessages(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"A"},{"role":"assistant","content":"X"},{"role":"user","content":"B"}]}`
	p, _ := Parse([]byte(body))
	msgs := p.AllUserMessages()
	if len(msgs) != 2 {
		t.Errorf("expected 2 user messages, got %d", len(msgs))
	}
}

func TestTotalChars(t *testing.T) {
	body := `{"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"World!"}]}`
	p, _ := Parse([]byte(body))
	total := p.TotalChars()
	if total == 0 {
		t.Error("TotalChars() should be > 0")
	}
}

func TestMessagesWithContentBlocks(t *testing.T) {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"Hello"},{"type":"image","source":{"type":"base64","data":"xyz"}}]}]}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if len(p.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(p.Messages))
	}
	// Ensure we preserve content blocks.
	content := string(p.Messages[0].Content)
	if !strings.Contains(content, "image") {
		t.Error("content should preserve image block")
	}
}

func TestMessagesWithComplexContent(t *testing.T) {
	body := `{"messages":[{"role":"assistant","content":[{"type":"text","text":"Sure"},{"type":"tool_use","id":"x","name":"read_file","input":{"path":"test.txt"}}]}]}`
	p, err := Parse([]byte(body))
	if err != nil {
		t.Fatalf("Parse() failed: %v", err)
	}
	if len(p.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(p.Messages))
	}
	content := string(p.Messages[0].Content)
	if !strings.Contains(content, "tool_use") {
		t.Error("content should preserve tool_use block")
	}
}