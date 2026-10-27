package router

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/DrXOps/llm-router/internal/config"
)

// LoadedRule pairs a parsed rule definition with the file it came from.
type LoadedRule struct {
	Def  config.RuleDef
	File string
}

// LoadRulesDir scans the given directory for YAML rule files and loads them.
// If dir is empty or does not exist, returns empty slice and no error.
func LoadRulesDir(dir string) ([]LoadedRule, error) {
	if dir == "" {
		return nil, nil
	}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // directory doesn't exist, no rules loaded
		}
		return nil, fmt.Errorf("stat rules directory %s: %w", dir, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read rules directory %s: %w", dir, err)
	}

	var rules []LoadedRule

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read rule file %s: %w", path, err)
		}

		// Can be a single rule or a list of rules.
		var single config.RuleDef
		var list []config.RuleDef

		if err := yaml.Unmarshal(data, &single); err == nil && single.Type != "" {
			rules = append(rules, LoadedRule{Def: single, File: name})
		} else if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
			for _, r := range list {
				rules = append(rules, LoadedRule{Def: r, File: name})
			}
		} else {
			// Try single again — if it fails, log but skip.
			continue
		}
	}

	// Sort by priority, then by filename.
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Def.Priority != rules[j].Def.Priority {
			return rules[i].Def.Priority < rules[j].Def.Priority
		}
		return rules[i].File < rules[j].File
	})

	return rules, nil
}

// LoadBuiltinRuleDefs wraps config.routing.rules into LoadedRule list.
func LoadBuiltinRuleDefs(rules []config.RuleDef) []LoadedRule {
	loaded := make([]LoadedRule, len(rules))
	for i, r := range rules {
		loaded[i] = LoadedRule{Def: r, File: "(config)"}
	}
	return loaded
}

// ParsePriorityFromFilename extracts a numeric priority from a filename prefix.
// E.g., "10-tool-calls.yaml" -> 10, "custom-rule.yaml" -> 100.
func ParsePriorityFromFilename(name string) int {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	parts := strings.SplitN(base, "-", 2)
	if len(parts) > 0 {
		var prio int
		if _, err := fmt.Sscanf(parts[0], "%d", &prio); err == nil {
			return prio
		}
	}
	return 100
}

// BuildRouterFromDefs creates a Router from a list of loaded rule definitions.
func BuildRouterFromDefs(loaded []LoadedRule, cfg *config.Config) (*Router, error) {
	rt := NewRouter()

	// Build valid targets set.
	validTargets := make(map[string]bool)
	for key := range cfg.Models {
		validTargets[key] = true
	}

	// Build phrase sets map.
	phraseSets := make(map[string][]string)
	for _, ps := range cfg.Phrases {
		phraseSets[ps.Name] = ps.Phrases
	}

	for _, lr := range loaded {
		def := lr.Def

		// Fill priority from filename if not set.
		if def.Priority == 0 {
			def.Priority = ParsePriorityFromFilename(lr.File)
		}

		// Enable by default if not specified.
		if !def.Enabled && lr.File != "(config)" {
			// Only rules from files can be disabled; built-in rules from config
			// are filtered by BuildBuiltinRule's Enabled check.
		}

		rule, err := BuildBuiltinRule(def, validTargets, phraseSets)
		if err != nil {
			return nil, fmt.Errorf("build rule %q from %s: %w", def.Name, lr.File, err)
		}
		if rule != nil {
			rt.AddRule(rule)
		}
	}

	return rt, nil
}

// LoadAndBuildRouter loads rules from config and rules_dir, then builds a Router.
func LoadAndBuildRouter(cfg *config.Config) (*Router, error) {
	var allRules []LoadedRule

	// Load built-in rules from config if enabled.
	if cfg.Routing.IncludeBuiltinRules {
		builtin := LoadBuiltinRuleDefs(cfg.Routing.Rules)
		allRules = append(allRules, builtin...)
	}

	// Load rules from directory.
	dirRules, err := LoadRulesDir(cfg.Routing.RulesDir)
	if err != nil {
		return nil, fmt.Errorf("load rules from %s: %w", cfg.Routing.RulesDir, err)
	}
	allRules = append(allRules, dirRules...)

	// Sort all rules by priority.
	sort.Slice(allRules, func(i, j int) bool {
		if allRules[i].Def.Priority != allRules[j].Def.Priority {
			return allRules[i].Def.Priority < allRules[j].Def.Priority
		}
		return allRules[i].File < allRules[j].File
	})

	return BuildRouterFromDefs(allRules, cfg)
}
