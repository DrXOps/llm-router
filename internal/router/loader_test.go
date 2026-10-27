package router

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/user/llm-router/internal/config"
)

func writeYAML(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		panic(err)
	}
}

func TestLoadRulesDirEmpty(t *testing.T) {
	rules, err := LoadRulesDir("")
	if err != nil {
		t.Fatalf("LoadRulesDir('') failed: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for empty dir, got %d", len(rules))
	}
}

func TestLoadRulesDirNonExistent(t *testing.T) {
	rules, err := LoadRulesDir("/nonexistent/path")
	if err != nil {
		t.Fatalf("LoadRulesDir(nonexistent) failed: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for nonexistent dir, got %d", len(rules))
	}
}

func TestLoadRulesDirSingleFile(t *testing.T) {
	dir := t.TempDir()
	writeYAML(filepath.Join(dir, "10-test.yaml"), "name: test_rule\npriority: 10\nenabled: true\ntype: always\ntarget: haiku\ndescription: Test rule\n")

	rules, err := LoadRulesDir(dir)
	if err != nil {
		t.Fatalf("LoadRulesDir failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Def.Name != "test_rule" {
		t.Errorf("expected name test_rule, got %q", rules[0].Def.Name)
	}
	if rules[0].Def.Priority != 10 {
		t.Errorf("expected priority 10, got %d", rules[0].Def.Priority)
	}
}

func TestLoadRulesDirMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(filepath.Join(dir, "10-first.yaml"), "name: first\npriority: 10\nenabled: true\ntype: always\ntarget: haiku\n")
	writeYAML(filepath.Join(dir, "20-second.yaml"), "name: second\npriority: 20\nenabled: true\ntype: always\ntarget: deepseek\n")

	rules, err := LoadRulesDir(dir)
	if err != nil {
		t.Fatalf("LoadRulesDir failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Def.Name != "first" || rules[1].Def.Name != "second" {
		t.Errorf("expected order first, second, got %q, %q", rules[0].Def.Name, rules[1].Def.Name)
	}
}

func TestLoadRulesDirOnlyYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(filepath.Join(dir, "note.txt"), "not a rule")
	writeYAML(filepath.Join(dir, "rule.yaml"), "name: r\npriority: 1\nenabled: true\ntype: always\ntarget: haiku\n")

	rules, err := LoadRulesDir(dir)
	if err != nil {
		t.Fatalf("LoadRulesDir failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
}

func TestLoadRulesDirList(t *testing.T) {
	dir := t.TempDir()
	writeYAML(filepath.Join(dir, "rules.yaml"), "- name: rule_a\n  priority: 10\n  enabled: true\n  type: always\n  target: haiku\n- name: rule_b\n  priority: 20\n  enabled: true\n  type: always\n  target: deepseek\n")

	rules, err := LoadRulesDir(dir)
	if err != nil {
		t.Fatalf("LoadRulesDir failed: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestParsePriorityFromFilename(t *testing.T) {
	cases := []struct {
		name     string
		expected int
	}{
		{"10-tool-calls.yaml", 10},
		{"20-keywords.yaml", 20},
		{"5-test.yaml", 5},
		{"custom-rule.yaml", 100},
		{"no-prefix.yaml", 100},
		{"abc-rule.yaml", 100},
	}

	for _, c := range cases {
		got := ParsePriorityFromFilename(c.name)
		if got != c.expected {
			t.Errorf("ParsePriorityFromFilename(%q) = %d, expected %d", c.name, got, c.expected)
		}
	}
}

func TestLoadBuiltinRuleDefs(t *testing.T) {
	defs := []config.RuleDef{
		{Name: "a", Type: "always", Target: "haiku"},
		{Name: "b", Type: "always", Target: "deepseek"},
	}
	loaded := LoadBuiltinRuleDefs(defs)
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded rules, got %d", len(loaded))
	}
	if loaded[0].File != "(config)" {
		t.Errorf("expected file (config), got %q", loaded[0].File)
	}
}

func TestBuildRouterFromDefs(t *testing.T) {
	cfg := config.DefaultConfig()
	loaded := []LoadedRule{
		{Def: config.RuleDef{Name: "fallback", Priority: 9999, Enabled: true, Type: "always", Target: "haiku"}, File: "(config)"},
	}

	rt, err := BuildRouterFromDefs(loaded, cfg)
	if err != nil {
		t.Fatalf("BuildRouterFromDefs failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil router")
	}

	ctx := NewPayloadCtx(map[string]string{"haiku": "haiku-model"})
	result := rt.Eval(ctx)
	if !result.Match {
		t.Error("expected a match")
	}
	if result.Target != "haiku" {
		t.Errorf("expected target haiku, got %q", result.Target)
	}
}

func TestBuildRouterFromDefsBadTarget(t *testing.T) {
	cfg := config.DefaultConfig()
	loaded := []LoadedRule{
		{Def: config.RuleDef{Name: "bad", Priority: 1, Enabled: true, Type: "always", Target: "nonexistent"}, File: "bad.yaml"},
	}

	_, err := BuildRouterFromDefs(loaded, cfg)
	if err == nil {
		t.Error("BuildRouterFromDefs should fail with bad target")
	}
}

func TestSortingPriorityThenFilename(t *testing.T) {
	rules := []LoadedRule{
		{Def: config.RuleDef{Name: "z", Priority: 10}, File: "c.yaml"},
		{Def: config.RuleDef{Name: "a", Priority: 10}, File: "a.yaml"},
		{Def: config.RuleDef{Name: "m", Priority: 5}, File: "b.yaml"},
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Def.Priority != rules[j].Def.Priority {
			return rules[i].Def.Priority < rules[j].Def.Priority
		}
		return rules[i].File < rules[j].File
	})

	if rules[0].Def.Name != "m" {
		t.Errorf("expected first rule m (priority 5), got %q", rules[0].Def.Name)
	}
	if rules[1].Def.Name != "a" {
		t.Errorf("expected second rule a (priority 10, file a.yaml), got %q", rules[1].Def.Name)
	}
	if rules[2].Def.Name != "z" {
		t.Errorf("expected third rule z (priority 10, file c.yaml), got %q", rules[2].Def.Name)
	}
}

func TestLoadAndBuildRouter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = t.TempDir()

	writeYAML(filepath.Join(cfg.Routing.RulesDir, "custom.yaml"), "name: custom\npriority: 100\nenabled: true\ntype: always\ntarget: deepseek\ndescription: custom\n")

	rt, err := LoadAndBuildRouter(cfg)
	if err != nil {
		t.Fatalf("LoadAndBuildRouter failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil router")
	}

	ctx := NewPayloadCtx(map[string]string{"haiku": "haiku-model", "deepseek": "deepseek-model"})
	result := rt.Eval(ctx)
	if !result.Match {
		t.Fatal("expected a match")
	}
	// custom.yaml has priority 100, built-in always_haiku_fallback has 9999.
	// So the custom rule should fire first.
	if result.Target != "deepseek" {
		t.Errorf("expected custom rule (deepseek) to fire first, got %q", result.Target)
	}
}

func TestLoadAndBuildRouterNoRulesDir(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Routing.IncludeBuiltinRules = true
	cfg.Routing.RulesDir = ""

	rt, err := LoadAndBuildRouter(cfg)
	if err != nil {
		t.Fatalf("LoadAndBuildRouter failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil router")
	}
	if len(rt.Rules()) == 0 {
		t.Error("expected at least built-in rules")
	}
}

func TestLoadAndBuildRouterOnlyBuiltin(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Routing.RulesDir = "/nonexistent"
	cfg.Routing.IncludeBuiltinRules = true

	rt, err := LoadAndBuildRouter(cfg)
	if err != nil {
		t.Fatalf("LoadAndBuildRouter failed: %v", err)
	}
	if len(rt.Rules()) == 0 {
		t.Error("expected built-in rules to be loaded")
	}
}

func TestRulesDirYMLSupport(t *testing.T) {
	dir := t.TempDir()
	writeYAML(filepath.Join(dir, "rule.yml"), "name: yml_rule\npriority: 1\nenabled: true\ntype: always\ntarget: haiku\n")

	rules, err := LoadRulesDir(dir)
	if err != nil {
		t.Fatalf("LoadRulesDir failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule from .yml, got %d", len(rules))
	}
}