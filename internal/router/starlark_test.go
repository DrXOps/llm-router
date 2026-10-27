package router

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/user/llm-router/internal/config"
)

func testStarlarkCtx() *PayloadCtx {
	return &PayloadCtx{
		ModelIDs:        map[string]string{"haiku": "claude-haiku-5-5", "deepseek": "deepseek-v4-pro"},
		HasToolCalls:    false,
		LastUserMessage: "What is the weather like today?",
		AllUserMessages: []string{"Hello", "What is the weather like today?"},
		TotalChars:      45,
		ApproxTokens:    11,
		MessageCount:    2,
	}
}

func TestStarlarkRuleMatch(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    if ctx.get(\"has_tool_calls\", False):\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"tool calls detected\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_match", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	ctx.HasToolCalls = true
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when has_tool_calls is true, reason: %q", result.Reason)
	}
	if result.Target != "deepseek" {
		t.Errorf("expected target deepseek, got %q", result.Target)
	}
}

func TestStarlarkRuleNoMatch(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    if ctx.get(\"has_tool_calls\", False):\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"tools\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_nomatch", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	ctx.HasToolCalls = false
	result := rule.Eval(ctx)
	if result.Match {
		t.Error("expected no match when has_tool_calls is false")
	}
}

func TestStarlarkRuleWithKeywords(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    msgs = ctx.get(\"all_user_messages\", [])\n" +
		"    for msg in msgs:\n" +
		"        if \"TODO\" in msg:\n" +
		"            return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"TODO found\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_keyword", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	ctx.AllUserMessages = []string{"TODO: fix this bug", "Hello"}
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when TODO is in messages, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleApproxTokens(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    if ctx.get(\"approx_tokens\", 0) > 100:\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"long context\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_tokens", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	ctx.ApproxTokens = 200
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when approx_tokens > 100, reason: %q", result.Reason)
	}

	ctx2 := testStarlarkCtx()
	ctx2.ApproxTokens = 10
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("expected no match when approx_tokens <= 100")
	}
}

func TestStarlarkRuleMessageCount(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    if ctx.get(\"message_count\", 0) > 5:\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"many messages\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_count", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	ctx.MessageCount = 10
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when message_count > 5, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleModelIDs(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    mid = ctx.get(\"model_ids\", {})\n" +
		"    haiku_id = mid.get(\"haiku\", \"\")\n" +
		"    if haiku_id == \"claude-haiku-5-5\":\n" +
		"        return {\"match\": True, \"target\": \"haiku\", \"reason\": \"haiku model verified\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_models", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when model_ids contains haiku, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleCompileError(t *testing.T) {
	src := "def rule(payload, ctx):\n  syntax error\n"
	cfg := config.DefaultConfig()
	_, err := NewStarlarkRule("bad_syntax", 100, src, cfg)
	if err == nil {
		t.Error("NewStarlarkRule should fail with syntax error")
	}
}

func TestStarlarkRuleNoRuleFunction(t *testing.T) {
	src := "x = 42\n"
	cfg := config.DefaultConfig()
	_, err := NewStarlarkRule("no_func", 100, src, cfg)
	if err == nil {
		t.Error("NewStarlarkRule should fail without rule() function")
	}
}

func TestStarlarkRuleTimeout(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    for i in range(100000000):\n" +
		"        r = i * i\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("infinite", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if result.Match {
		t.Error("expected no match for timeout")
	}
	if result.Reason == "" {
		t.Error("expected timeout reason message")
	}
}

func TestStarlarkRuleInvalidReturnType(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    return \"string instead of dict\"\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("bad_return", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if result.Match {
		t.Error("expected no match for invalid return type")
	}
}

func TestStarlarkRuleRuntimeError(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    return 1 // 0\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("runtime_err", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if result.Match {
		t.Error("expected no match for runtime error")
	}
}

func TestStarlarkRuleLastUserMessage(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    msg = ctx.get(\"last_user_message\", \"\")\n" +
		"    if \"weather\" in msg:\n" +
		"        return {\"match\": True, \"target\": \"haiku\", \"reason\": \"simple weather query\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("test_last_msg", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when last message contains 'weather', reason: %q", result.Reason)
	}
}

func TestLoadStarlarkFiles(t *testing.T) {
	dir := t.TempDir()
	src := "def rule(payload, ctx):\n" +
		"    return {\"match\": True, \"target\": \"haiku\", \"reason\": \"test\"}\n"
	os.WriteFile(filepath.Join(dir, "10-test.star"), []byte(src), 0644)

	cfg := config.DefaultConfig()
	rules, err := LoadStarlarkFiles(dir, cfg)
	if err != nil {
		t.Fatalf("LoadStarlarkFiles failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestLoadStarlarkFilesEmptyDir(t *testing.T) {
	cfg := config.DefaultConfig()
	rules, err := LoadStarlarkFiles("", cfg)
	if err != nil {
		t.Fatalf("LoadStarlarkFiles('') failed: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestLoadStarlarkFilesNonExistentDir(t *testing.T) {
	cfg := config.DefaultConfig()
	rules, err := LoadStarlarkFiles("/nonexistent", cfg)
	if err != nil {
		t.Fatalf("LoadStarlarkFiles(nonexistent) failed: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestLoadStarlarkFilesIgnoresNonStar(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "rule.star"),
		[]byte("def rule(payload, ctx):\n    return {\"match\": False}\n"), 0644)

	cfg := config.DefaultConfig()
	rules, err := LoadStarlarkFiles(dir, cfg)
	if err != nil {
		t.Fatalf("LoadStarlarkFiles failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestLoadStarlarkFilesBrokenSkip(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "good.star"),
		[]byte("def rule(payload, ctx):\n    return {\"match\": False}\n"), 0644)
	os.WriteFile(filepath.Join(dir, "bad.star"), []byte("syntax error!!!"), 0644)

	cfg := config.DefaultConfig()
	cfg.Routing.StrictValidation = false
	rules, err := LoadStarlarkFiles(dir, cfg)
	if err != nil {
		t.Fatalf("LoadStarlarkFiles failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule (skip broken), got %d", len(rules))
	}
}

func TestLoadStarlarkFilesBrokenStrict(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.star"), []byte("syntax error!!!"), 0644)

	cfg := config.DefaultConfig()
	cfg.Routing.StrictValidation = true
	_, err := LoadStarlarkFiles(dir, cfg)
	if err == nil {
		t.Error("LoadStarlarkFiles should fail with StrictValidation=true on broken file")
	}
}

func TestStarlarkRuleLogFunction(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    ctx.get(\"log\", lambda x: x)(\"debug message\")\n" +
		"    return {\"match\": True, \"target\": \"haiku\", \"reason\": \"log test\"}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("log_test", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match even with log call, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleWithPayload(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    model = payload.get(\"model\", \"\")\n" +
		"    if model == \"\":\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"no model set yet\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("payload_test", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when payload model is empty, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleWithPayloadMessages(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    msgs = payload.get(\"messages\", [])\n" +
		"    if len(msgs) == 0:\n" +
		"        return {\"match\": True, \"target\": \"haiku\", \"reason\": \"no messages\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("payload_msgs", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match {
		t.Errorf("expected match when payload messages is empty, reason: %q", result.Reason)
	}
}

func TestStarlarkRuleComplexCondition(t *testing.T) {
	src := "def rule(payload, ctx):\n" +
		"    msg = ctx.get(\"last_user_message\", \"\")\n" +
		"    count = ctx.get(\"message_count\", 0)\n" +
		"    tokens = ctx.get(\"approx_tokens\", 0)\n" +
		"    tools = ctx.get(\"has_tool_calls\", False)\n" +
		"    if count > 5 and tokens > 100:\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"long complex conversation\"}\n" +
		"    if \"weather\" in msg and tokens < 50:\n" +
		"        return {\"match\": True, \"target\": \"haiku\", \"reason\": \"simple weather query\"}\n" +
		"    if tools:\n" +
		"        return {\"match\": True, \"target\": \"deepseek\", \"reason\": \"tools in conversation\"}\n" +
		"    return {\"match\": False}\n"
	cfg := config.DefaultConfig()
	rule, err := NewStarlarkRule("complex", 100, src, cfg)
	if err != nil {
		t.Fatalf("NewStarlarkRule failed: %v", err)
	}

	// Test: simple weather query → haiku
	ctx := testStarlarkCtx()
	result := rule.Eval(ctx)
	if !result.Match || result.Target != "haiku" {
		t.Errorf("expected haiku for weather query, got match=%v target=%q", result.Match, result.Target)
	}

	// Test: long complex → deepseek
	ctx2 := testStarlarkCtx()
	ctx2.MessageCount = 10
	ctx2.ApproxTokens = 200
	result2 := rule.Eval(ctx2)
	if !result2.Match || result2.Target != "deepseek" {
		t.Errorf("expected deepseek for long complex, got match=%v target=%q", result2.Match, result2.Target)
	}
}