package sanitizer

import (
	"encoding/json"
	"testing"
)

func TestSanitizeForModelHaiku(t *testing.T) {
	body := []byte(`{"model":"claude-haiku-5-5","messages":[{"role":"user","content":"Hello <thinking>secret</thinking> world"}]}`)
	result, err := SanitizeForModel(body, "haiku")
	if err != nil {
		t.Fatalf("SanitizeForModel(haiku) failed: %v", err)
	}
	// Haiku should pass through unchanged.
	if string(result) != string(body) {
		t.Errorf("haiku should pass through unchanged, got: %s", string(result))
	}
}

func TestSanitizeForModelDeepSeek(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"Hello <thinking>secret</thinking> world"}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel(deepseek) failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].(string)

	if content != "Hello  world" {
		t.Errorf("expected 'Hello  world', got %q", content)
	}
}

func TestSanitizeNoThinkingBlocks(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"Hello world"}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].(string)

	if content != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", content)
	}
}

func TestSanitizeMultipleThinkingBlocks(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"A <thinking>secret1</thinking> B <thinking>secret2</thinking> C"}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].(string)

	if content != "A  B  C" {
		t.Errorf("expected 'A  B  C', got %q", content)
	}
}

func TestSanitizeNestedThinking(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":"<thinking>outer <thinking>inner</thinking> still outer</thinking> visible"}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].(string)

	if content != " visible" {
		t.Errorf("expected ' visible', got %q", content)
	}
}

func TestSanitizeUnclosedThinking(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":"<thinking>no closing tag"}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].(string)

	if content != "" {
		t.Errorf("expected empty string for unclosed thinking, got %q", content)
	}
}

func TestSanitizeContentBlocks(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"text","text":"Hello <thinking>hidden</thinking> world"},{"type":"tool_use","id":"x","name":"test"}]}]}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	msgs := parsed["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)
	textBlock := content[0].(map[string]any)
	text := textBlock["text"].(string)

	if text != "Hello  world" {
		t.Errorf("expected 'Hello  world', got %q", text)
	}

	// tool_use block should be preserved.
	toolBlock := content[1].(map[string]any)
	if toolBlock["type"] != "tool_use" {
		t.Errorf("expected tool_use block to be preserved, got type=%v", toolBlock["type"])
	}
}

func TestSanitizePreservesJSON(t *testing.T) {
	body := []byte(`{"stream":true,"max_tokens":4096,"messages":[{"role":"user","content":"<thinking>sec</thinking>hello"}],"temperature":0.5}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(result, &parsed)
	if parsed["stream"] != true {
		t.Error("stream field should be preserved")
	}
	if parsed["max_tokens"] != float64(4096) {
		t.Error("max_tokens should be preserved")
	}
	if parsed["temperature"] != 0.5 {
		t.Error("temperature should be preserved")
	}
}

func TestContainsThinkingBlocks(t *testing.T) {
	if ContainsThinkingBlocks([]byte("hello world")) {
		t.Error("should be false for plain text")
	}
	if !ContainsThinkingBlocks([]byte("hello <thinking>secret</thinking>")) {
		t.Error("should be true when thinking blocks present")
	}
	if ContainsThinkingBlocks([]byte("<think>not thinking</think>")) {
		t.Error("should be false for <think> without ing")
	}
}

func TestSanitizeInvalidJSON(t *testing.T) {
	// We only care that the function doesn't panic on invalid input.
	body := []byte(`{invalid}`)
	result, err := stripThinkingBlocks(body)
	_ = result
	_ = err
	// Either error or valid output is acceptable; no crash is the requirement.
}

func TestSanitizeDeepSeekNoMessages(t *testing.T) {
	body := []byte(`{"model":"deepseek"}`)
	result, err := SanitizeForModel(body, "deepseek")
	if err != nil {
		t.Fatalf("SanitizeForModel failed: %v", err)
	}
	if !json.Valid(result) {
		t.Error("result should be valid JSON")
	}
}