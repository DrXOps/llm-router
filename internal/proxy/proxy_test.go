package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DrXOps/llm-router/internal/config"
	"github.com/DrXOps/llm-router/internal/router"
)

func TestHandlerRejectsNonPost(t *testing.T) {
	cfg := config.DefaultConfig()
	rt, _ := router.LoadAndBuildRouter(cfg)
	h := Handler(cfg, router.NewAtomicRouter(rt), nil)

	ts := httptest.NewServer(h)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/messages")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for GET, got %d", resp.StatusCode)
	}
}

func TestHandlerRejectsWrongPath(t *testing.T) {
	cfg := config.DefaultConfig()
	rt, _ := router.LoadAndBuildRouter(cfg)
	h := Handler(cfg, router.NewAtomicRouter(rt), nil)

	ts := httptest.NewServer(h)
	defer ts.Close()

	body := strings.NewReader(`{"messages":[{"role":"user","content":"hello"}]}`)
	resp, err := http.Post(ts.URL+"/v1/other", "application/json", body)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for wrong path, got %d", resp.StatusCode)
	}
}

func TestHandlerProxiesRequest(t *testing.T) {
	// Start a test upstream server.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the proxied request.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/messages" {
			t.Errorf("expected /v1/messages, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(body, &parsed)

		// Verify model was rewritten.
		model, ok := parsed["model"].(string)
		if !ok || model == "" {
			t.Errorf("expected model in proxied request, got %v", parsed["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"Hello!"}]}`))
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = upstream.URL
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = ""
	rt, _ := router.LoadAndBuildRouter(cfg)

	h := Handler(cfg, router.NewAtomicRouter(rt), nil)
	ts := httptest.NewServer(h)
	defer ts.Close()

	body := `{"model":"original-model","messages":[{"role":"user","content":"Hi"}],"max_tokens":100}`
	resp, err := http.Post(ts.URL+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(respBody, &result)
	if result["content"] == nil {
		t.Errorf("expected upstream response, got %v", string(respBody))
	}
}

func TestHandlerProxiesWithToolCalls(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(body, &parsed)

		model, _ := parsed["model"].(string)
		if model != "deepseek-v4-pro" {
			t.Errorf("expected deepseek-v4-pro for tool calls, got %q", model)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"Done"}]}`))
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = upstream.URL
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = ""
	// Add a has_tool_calls rule at lower priority than the fallback.
	cfg.Routing.Rules = []config.RuleDef{
		{
			Name:     "tools_to_deepseek",
			Priority: 10,
			Enabled:  true,
			Type:     "has_tool_calls",
			Target:   "deepseek",
		},
		{
			Name:     "fallback_haiku",
			Priority: 9999,
			Enabled:  true,
			Type:     "always",
			Target:   "haiku",
		},
	}
	rt, _ := router.LoadAndBuildRouter(cfg)

	h := Handler(cfg, router.NewAtomicRouter(rt), nil)
	ts := httptest.NewServer(h)
	defer ts.Close()

	body := `{"model":"old","messages":[{"role":"user","content":"Hi"},{"role":"assistant","content":[{"type":"tool_use","id":"1","name":"test"}]}],"max_tokens":100}`
	resp, err := http.Post(ts.URL+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandlerInvalidBody(t *testing.T) {
	cfg := config.DefaultConfig()
	rt, _ := router.LoadAndBuildRouter(cfg)
	h := Handler(cfg, router.NewAtomicRouter(rt), nil)

	ts := httptest.NewServer(h)
	defer ts.Close()

	body := strings.NewReader(`{invalid json}`)
	resp, err := http.Post(ts.URL+"/v1/messages", "application/json", body)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d", resp.StatusCode)
	}
}

func TestHandlerProxiesAuthHeader(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		if apiKey != "test-key-123" {
			t.Errorf("expected x-api-key header to be preserved, got %q", apiKey)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = upstream.URL
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = ""
	rt, _ := router.LoadAndBuildRouter(cfg)

	h := Handler(cfg, router.NewAtomicRouter(rt), nil)
	ts := httptest.NewServer(h)
	defer ts.Close()

	reqBody := `{"messages":[{"role":"user","content":"test"}]}`
	req, err := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("NewRequest failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", "test-key-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHandlerPreservesExtraFields(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		json.Unmarshal(body, &parsed)

		if parsed["max_tokens"] != float64(200) {
			t.Errorf("expected max_tokens=200, got %v", parsed["max_tokens"])
		}
		if parsed["stream"] != true {
			t.Errorf("expected stream=true, got %v", parsed["stream"])
		}
		if parsed["temperature"] != 0.3 {
			t.Errorf("expected temperature=0.3, got %v", parsed["temperature"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()

	cfg := config.DefaultConfig()
	cfg.Upstream.BaseURL = upstream.URL
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = ""
	rt, _ := router.LoadAndBuildRouter(cfg)

	h := Handler(cfg, router.NewAtomicRouter(rt), nil)
	ts := httptest.NewServer(h)
	defer ts.Close()

	body := `{"model":"old","messages":[{"role":"user","content":"test"}],"max_tokens":200,"stream":true,"temperature":0.3}`
	resp, err := http.Post(ts.URL+"/v1/messages", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("truncate should not truncate short strings")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Error("truncate should truncate and add ...")
	}
}
