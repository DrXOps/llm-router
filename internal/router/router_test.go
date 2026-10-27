package router

import (
	"testing"

	"github.com/user/llm-router/internal/config"
)

// testPayloadCtx builds a PayloadCtx from payload-like data for testing.
func testPayloadCtx(hasToolCalls bool, lastUserMsg string, allUserMsgs []string, totalChars, approxTokens int) *PayloadCtx {
	if allUserMsgs == nil {
		allUserMsgs = []string{}
	}
	return &PayloadCtx{
		ModelIDs:         map[string]string{"haiku": "claude-haiku-5-5", "deepseek": "deepseek-v4-pro"},
		HasToolCalls:     hasToolCalls,
		LastUserMessage:  lastUserMsg,
		AllUserMessages:  allUserMsgs,
		TotalChars:       totalChars,
		ApproxTokens:     approxTokens,
	}
}

func TestAlwaysRule(t *testing.T) {
	rule := NewAlwaysRule("test_always", 100, "haiku")
	ctx := testPayloadCtx(false, "hello", nil, 5, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("always rule should always match")
	}
	if result.Target != "haiku" {
		t.Errorf("expected target haiku, got %q", result.Target)
	}
}

func TestHasToolCallsRule(t *testing.T) {
	rule := NewHasToolCallsRule("test_tools", 10, "deepseek")

	// Should match when tools present
	ctx := testPayloadCtx(true, "hello", nil, 5, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("has_tool_calls should match when tools present")
	}
	if result.Target != "deepseek" {
		t.Errorf("expected target deepseek, got %q", result.Target)
	}

	// Should not match when no tools
	ctx2 := testPayloadCtx(false, "hello", nil, 5, 1)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("has_tool_calls should not match when no tools")
	}
}

func TestContextLengthRule(t *testing.T) {
	rule := NewContextLengthRule("test_context", 10, "deepseek", 100)

	// Should match when above threshold
	ctx := testPayloadCtx(false, "hello", nil, 500, 150)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("context_length should match when above threshold")
	}

	// Should not match when below threshold
	ctx2 := testPayloadCtx(false, "hello", nil, 10, 2)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("context_length should not match when below threshold")
	}
}

func TestShortQuestionRule(t *testing.T) {
	rule := NewShortQuestionRule("test_short", 10, "haiku", 5)

	// Short message should match
	ctx := testPayloadCtx(false, "hi", nil, 2, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("short_question should match short message")
	}

	// Long message should not match
	ctx2 := testPayloadCtx(false, "This is a very long question with many tokens", nil, 50, 15)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("short_question should not match long message")
	}

	// Empty message should not match
	ctx3 := testPayloadCtx(false, "", nil, 0, 0)
	result3 := rule.Eval(ctx3)
	if result3.Match {
		t.Error("short_question should not match empty message")
	}
}

func TestKeywordMatchRule(t *testing.T) {
	phrases := []string{"refactor", "architecture", "technical debt"}
	rule := NewKeywordMatchRule("test_kw", 20, "deepseek", phrases)

	// Should match when keyword present
	ctx := testPayloadCtx(false, "we need to refactor this", []string{"we need to refactor this"}, 30, 10)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("keyword_match should match when keyword present")
	}
	if result.Target != "deepseek" {
		t.Errorf("expected target deepseek, got %q", result.Target)
	}

	// Should not match when keyword absent
	ctx2 := testPayloadCtx(false, "hello world", []string{"hello world"}, 11, 3)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("keyword_match should not match when keyword absent")
	}
}

func TestKeywordMatchCaseInsensitive(t *testing.T) {
	phrases := []string{"Refactor"}
	rule := NewKeywordMatchRule("test_kw_ci", 20, "deepseek", phrases)

	ctx := testPayloadCtx(false, "Let's REFACTOR everything", []string{"Let's REFACTOR everything"}, 30, 10)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("keyword_match should be case-insensitive")
	}
}

func TestRegexMatchRule(t *testing.T) {
	rule, err := NewRegexMatchRule("test_regex", 30, "deepseek", `TODO|FIXME`, "any_user_message")
	if err != nil {
		t.Fatalf("NewRegexMatchRule failed: %v", err)
	}

	// Should match
	ctx := testPayloadCtx(false, "TODO: fix this", []string{"TODO: fix this"}, 15, 4)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("regex_match should match TODO")
	}

	// Should not match
	ctx2 := testPayloadCtx(false, "everything is fine", []string{"everything is fine"}, 18, 5)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("regex_match should not match plain text")
	}
}

func TestRegexMatchLastUserMessage(t *testing.T) {
	rule, err := NewRegexMatchRule("test_regex_last", 30, "deepseek", `error`, "last_user_message")
	if err != nil {
		t.Fatalf("NewRegexMatchRule failed: %v", err)
	}

	// Should match
	ctx := testPayloadCtx(false, "I got an error", []string{"I got an error"}, 15, 4)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("regex_match should match 'error' in last user message")
	}

	// Should not match
	ctx2 := testPayloadCtx(false, "all good", []string{"all good"}, 8, 2)
	result2 := rule.Eval(ctx2)
	if result2.Match {
		t.Error("regex_match should not match 'all good'")
	}
}

func TestRegexMatchInvalidPattern(t *testing.T) {
	_, err := NewRegexMatchRule("bad_regex", 30, "haiku", `[invalid`, "any_user_message")
	if err == nil {
		t.Error("NewRegexMatchRule should fail with invalid regex")
	}
}

func TestRouterPriority(t *testing.T) {
	rt := NewRouter()

	// Rule with higher priority (lower number) should fire first.
	ruleLow := NewAlwaysRule("low_priority", 100, "haiku")
	ruleHigh := NewAlwaysRule("high_priority", 10, "deepseek")

	rt.AddRule(ruleLow, ruleHigh)

	ctx := testPayloadCtx(false, "test", nil, 4, 1)
	result := rt.Eval(ctx)

	if !result.Match {
		t.Fatal("expected a match")
	}
	if result.Target != "deepseek" {
		t.Errorf("expected high-priority rule (deepseek) to fire first, got %q", result.Target)
	}
}

func TestRouterFirstMatchOnly(t *testing.T) {
	rt := NewRouter()

	// Add two rules that would both match; should stop after first.
	rule1 := NewAlwaysRule("first", 10, "deepseek")
	rule2 := NewAlwaysRule("second", 20, "haiku")

	rt.AddRule(rule1, rule2)

	ctx := testPayloadCtx(false, "test", nil, 4, 1)
	result := rt.Eval(ctx)

	if result.Target != "deepseek" {
		t.Errorf("expected first rule (deepseek) to fire, got %q", result.Target)
	}
}

func TestRouterNoMatch(t *testing.T) {
	rt := NewRouter()
	// No rules — should return NoMatch.
	ctx := testPayloadCtx(false, "test", nil, 4, 1)
	result := rt.Eval(ctx)
	if result.Match {
		t.Error("expected no match when there are no rules")
	}
}

func TestBuildBuiltinAlways(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "always",
		Target:  "haiku",
	}
	validTargets := map[string]bool{"haiku": true}
	rule, err := BuildBuiltinRule(def, validTargets, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}
	if rule == nil {
		t.Fatal("expected non-nil rule")
	}

	ctx := testPayloadCtx(false, "x", nil, 1, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("always rule should match")
	}
}

func TestBuildBuiltinHasToolCalls(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "has_tool_calls",
		Target:  "deepseek",
	}
	validTargets := map[string]bool{"deepseek": true}
	rule, err := BuildBuiltinRule(def, validTargets, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}

	ctx := testPayloadCtx(true, "x", nil, 1, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("has_tool_calls should match")
	}
}

func TestBuildBuiltinContextLength(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "context_length",
		Target:  "deepseek",
		Params:  map[string]any{"threshold_tokens": 100},
	}
	validTargets := map[string]bool{"deepseek": true}
	rule, err := BuildBuiltinRule(def, validTargets, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}

	ctx := testPayloadCtx(false, "x", nil, 500, 200)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("context_length should match")
	}
}

func TestBuildBuiltinShortQuestion(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "short_question",
		Target:  "haiku",
		Params:  map[string]any{"max_tokens": 5},
	}
	validTargets := map[string]bool{"haiku": true}
	rule, err := BuildBuiltinRule(def, validTargets, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}

	ctx := testPayloadCtx(false, "hi", nil, 2, 1)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("short_question should match")
	}
}

func TestBuildBuiltinKeywordMatch(t *testing.T) {
	phraseSets := map[string][]string{"engineering": {"refactor", "architecture"}}
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "keyword_match",
		Target:  "deepseek",
		Params:  map[string]any{"phrase_set": "engineering"},
	}
	validTargets := map[string]bool{"deepseek": true}
	rule, err := BuildBuiltinRule(def, validTargets, phraseSets)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}

	ctx := testPayloadCtx(false, "let's refactor", []string{"let's refactor"}, 15, 4)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("keyword_match should match")
	}
}

func TestBuildBuiltinKeywordMatchUnknownSet(t *testing.T) {
	phraseSets := map[string][]string{"engineering": {"refactor"}}
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "keyword_match",
		Target:  "deepseek",
		Params:  map[string]any{"phrase_set": "nonexistent"},
	}
	validTargets := map[string]bool{"deepseek": true}
	_, err := BuildBuiltinRule(def, validTargets, phraseSets)
	if err == nil {
		t.Error("BuildBuiltinRule should fail with unknown phrase_set")
	}
}

func TestBuildBuiltinRegexMatch(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "regex_match",
		Target:  "deepseek",
		Params:  map[string]any{"pattern": "file:\\w+\\.go", "field": "any_user_message"},
	}
	validTargets := map[string]bool{"deepseek": true}
	rule, err := BuildBuiltinRule(def, validTargets, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}

	ctx := testPayloadCtx(false, "check file:main.go", []string{"check file:main.go"}, 20, 5)
	result := rule.Eval(ctx)
	if !result.Match {
		t.Error("regex_match should match file pattern")
	}
	if result.Reason == "" {
		t.Error("result should have a reason")
	}
}

func TestBuildBuiltinDisabledRule(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: false,
		Type:    "always",
		Target:  "haiku",
	}
	rule, err := BuildBuiltinRule(def, map[string]bool{"haiku": true}, nil)
	if err != nil {
		t.Fatalf("BuildBuiltinRule failed: %v", err)
	}
	if rule != nil {
		t.Error("disabled rule should return nil")
	}
}

func TestBuildBuiltinUnknownType(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "unknown_type",
		Target:  "haiku",
	}
	_, err := BuildBuiltinRule(def, map[string]bool{"haiku": true}, nil)
	if err == nil {
		t.Error("BuildBuiltinRule should fail with unknown type")
	}
}

func TestBuildBuiltinBadTarget(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "always",
		Target:  "nonexistent",
	}
	_, err := BuildBuiltinRule(def, map[string]bool{"haiku": true}, nil)
	if err == nil {
		t.Error("BuildBuiltinRule should fail with unknown target")
	}
}

func TestBuildBuiltinRegexNoPattern(t *testing.T) {
	def := config.RuleDef{
		Name:    "test",
		Enabled: true,
		Type:    "regex_match",
		Target:  "haiku",
	}
	_, err := BuildBuiltinRule(def, map[string]bool{"haiku": true}, nil)
	if err == nil {
		t.Error("BuildBuiltinRule should fail without pattern")
	}
}

func TestPayloadCtxCreation(t *testing.T) {
	ctx := NewPayloadCtx(nil)
	if ctx == nil {
		t.Fatal("NewPayloadCtx returned nil")
	}
	if ctx.ModelIDs == nil {
		t.Error("ModelIDs should not be nil")
	}

	ctx2 := NewPayloadCtx(map[string]string{"haiku": "haiku-model"})
	if ctx2.ModelIDs["haiku"] != "haiku-model" {
		t.Error("ModelIDs should contain haiku")
	}
}