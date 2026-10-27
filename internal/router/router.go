package router

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/DrXOps/llm-router/internal/config"
)

// Result represents the outcome of evaluating a rule.
type Result struct {
	Match  bool   `json:"match"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

// NoMatch returns a Result indicating no match.
func NoMatch() Result {
	return Result{}
}

// MatchResult returns a Result indicating a match with given target and reason.
func MatchResult(target, reason string) Result {
	return Result{Match: true, Target: target, Reason: reason}
}

// Rule defines the interface for a routing rule.
type Rule interface {
	// Name returns the rule's identifier.
	Name() string

	// Priority returns the rule's priority (lower = evaluated first).
	Priority() int

	// Eval evaluates the rule against the given payload context.
	// Returns a Result. If Match is false, the rule did not fire.
	Eval(p *PayloadCtx) Result
}

// RuleFunc adapts a function to the Rule interface.
type RuleFunc struct {
	name     string
	priority int
	fn       func(p *PayloadCtx) Result
}

func (r *RuleFunc) Name() string             { return r.name }
func (r *RuleFunc) Priority() int            { return r.priority }
func (r *RuleFunc) Eval(p *PayloadCtx) Result { return r.fn(p) }

// Router is an aggregator that evaluates rules in priority order.
type Router struct {
	rules []Rule
}

// NewRouter creates a new empty router.
func NewRouter() *Router {
	return &Router{rules: nil}
}

// AddRule adds one or more rules to the router.
func (rt *Router) AddRule(rules ...Rule) {
	rt.rules = append(rt.rules, rules...)
}

// Rules returns the current list of rules (sorted by priority).
func (rt *Router) Rules() []Rule {
	return rt.rules
}

// Eval evaluates all rules in priority order and returns the first match.
func (rt *Router) Eval(p *PayloadCtx) Result {
	// Sort by priority first (simple insertion sort since we rebuild order on add).
	rt.sortByPriority()

	for _, rule := range rt.rules {
		result := rule.Eval(p)
		if result.Match {
			return result
		}
	}
	return NoMatch()
}

func (rt *Router) sortByPriority() {
	// Simple bubble sort — the slice is typically very small (< 50 items).
	n := len(rt.rules)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if rt.rules[j].Priority() > rt.rules[j+1].Priority() {
				rt.rules[j], rt.rules[j+1] = rt.rules[j+1], rt.rules[j]
			}
		}
	}
}

// --- Built-in rule constructors ---

// NewAlwaysRule creates a rule that always matches with the given target.
func NewAlwaysRule(name string, priority int, target string) Rule {
	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			return MatchResult(target, fmt.Sprintf("always rule %q matched", name))
		},
	}
}

// NewHasToolCallsRule creates a rule that matches if the conversation has tool calls.
func NewHasToolCallsRule(name string, priority int, target string) Rule {
	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			if p.HasToolCalls {
				return MatchResult(target, "conversation contains tool calls")
			}
			return NoMatch()
		},
	}
}

// NewContextLengthRule creates a rule that matches if context exceeds a threshold.
func NewContextLengthRule(name string, priority int, target string, thresholdTokens int) Rule {
	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			if p.ApproxTokens >= thresholdTokens {
				return MatchResult(target, fmt.Sprintf("context %d tokens exceeds threshold %d", p.ApproxTokens, thresholdTokens))
			}
			return NoMatch()
		},
	}
}

// NewShortQuestionRule creates a rule that matches if the last user message is short.
func NewShortQuestionRule(name string, priority int, target string, maxTokens int) Rule {
	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			tokens := approxTokens(p.LastUserMessage)
			if tokens > 0 && tokens <= maxTokens {
				return MatchResult(target, fmt.Sprintf("last user message is short (%d tokens)", tokens))
			}
			return NoMatch()
		},
	}
}

// NewKeywordMatchRule creates a rule that matches if any trigger phrase appears in user messages.
func NewKeywordMatchRule(name string, priority int, target string, phrases []string) Rule {
	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			for _, msg := range p.AllUserMessages {
				lower := strings.ToLower(msg)
				for _, phrase := range phrases {
					if strings.Contains(lower, strings.ToLower(phrase)) {
						return MatchResult(target, fmt.Sprintf("keyword %q found in user message", phrase))
					}
				}
			}
			return NoMatch()
		},
	}
}

// NewRegexMatchRule creates a rule that matches if the pattern is found in user messages.
// field: "last_user_message" or "any_user_message"
func NewRegexMatchRule(name string, priority int, target string, pattern string, field string) (Rule, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compile regex %q: %w", pattern, err)
	}

	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			var texts []string
			switch field {
			case "last_user_message":
				if p.LastUserMessage != "" {
					texts = []string{p.LastUserMessage}
				}
			case "any_user_message":
				texts = p.AllUserMessages
			default:
				texts = p.AllUserMessages
			}

			for _, text := range texts {
				if re.MatchString(text) {
					return MatchResult(target, fmt.Sprintf("regex %q matched in user message", pattern))
				}
			}
			return NoMatch()
		},
	}, nil
}

// BuildBuiltinRule creates a built-in rule from a config RuleDef.
func BuildBuiltinRule(def config.RuleDef, validTargets map[string]bool, phraseSets map[string][]string) (Rule, error) {
	if !def.Enabled {
		return nil, nil
	}

	if def.Target != "" && !validTargets[def.Target] {
		return nil, fmt.Errorf("rule %q: unknown target %q", def.Name, def.Target)
	}

	switch def.Type {
	case "always":
		return NewAlwaysRule(def.Name, def.Priority, def.Target), nil

	case "has_tool_calls":
		return NewHasToolCallsRule(def.Name, def.Priority, def.Target), nil

	case "context_length":
		threshold := 4000 // default
		if def.Params != nil {
			if v, ok := def.Params["threshold_tokens"]; ok {
				switch t := v.(type) {
				case int:
					threshold = t
				case float64:
					threshold = int(t)
				}
			}
		}
		return NewContextLengthRule(def.Name, def.Priority, def.Target, threshold), nil

	case "short_question":
		maxTokens := 10 // default
		if def.Params != nil {
			if v, ok := def.Params["max_tokens"]; ok {
				switch t := v.(type) {
				case int:
					maxTokens = t
				case float64:
					maxTokens = int(t)
				}
			}
		}
		return NewShortQuestionRule(def.Name, def.Priority, def.Target, maxTokens), nil

	case "keyword_match":
		phraseSetName := ""
		if def.Params != nil {
			if v, ok := def.Params["phrase_set"].(string); ok {
				phraseSetName = v
			}
		}
		phrases, ok := phraseSets[phraseSetName]
		if !ok || len(phrases) == 0 {
			return nil, fmt.Errorf("rule %q: unknown or empty phrase_set %q", def.Name, phraseSetName)
		}
		return NewKeywordMatchRule(def.Name, def.Priority, def.Target, phrases), nil

	case "regex_match":
		pattern := ""
		field := "any_user_message"
		if def.Params != nil {
			if v, ok := def.Params["pattern"].(string); ok {
				pattern = v
			}
			if v, ok := def.Params["field"].(string); ok {
				field = v
			}
		}
		if pattern == "" {
			return nil, fmt.Errorf("rule %q: regex_match requires non-empty pattern", def.Name)
		}
		return NewRegexMatchRule(def.Name, def.Priority, def.Target, pattern, field)

	default:
		return nil, fmt.Errorf("rule %q: unknown type %q", def.Name, def.Type)
	}
}

// approxTokens is a local heuristic for token counting.
func approxTokens(s string) int {
	if s == "" {
		return 0
	}
	t := len([]rune(s)) / 4
	if t < 1 {
		t = 1
	}
	return t
}

// ValidateStarlarkRuleName checks if a filename looks like a Starlark rule.
func ValidateStarlarkRuleName(name string) bool {
	return strings.HasSuffix(name, ".star")
}
