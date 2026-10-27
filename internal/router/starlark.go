package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/DrXOps/llm-router/internal/config"
)

// starlarkTimeout is the maximum execution time for a single Starlark rule.
const starlarkTimeout = 100 * time.Millisecond

// NewStarlarkRule compiles a Starlark source file into a Rule.
// The source must define a function `rule(payload, ctx)` returning a dict.
func NewStarlarkRule(name string, priority int, src string, cfg *config.Config) (Rule, error) {
	predeclared := starlark.StringDict{
		"payload": starlark.None,
	}

	thread := &starlark.Thread{
		Name: "compile:" + name,
	}

	globals, err := starlark.ExecFile(thread, name, src, predeclared)
	if err != nil {
		return nil, fmt.Errorf("compile Starlark rule %q: %w", name, err)
	}

	ruleFn, ok := globals["rule"]
	if !ok {
		return nil, fmt.Errorf("Starlark rule %q: no function 'rule(payload, ctx)' defined", name)
	}

	ruleCallable, ok := ruleFn.(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("Starlark rule %q: 'rule' must be a function, got %T", name, ruleFn)
	}

	return &RuleFunc{
		name:     name,
		priority: priority,
		fn: func(p *PayloadCtx) Result {
			return evalStarlarkRule(ruleCallable, p, cfg)
		},
	}, nil
}

// evalStarlarkRule executes a compiled Starlark rule with a 100ms timeout.
func evalStarlarkRule(fn *starlark.Function, p *PayloadCtx, cfg *config.Config) Result {
	// Build the Starlark ctx dict.
	ctxDict := starlark.NewDict(10)
	ctxDict.SetKey(starlark.String("last_user_message"), starlark.String(p.LastUserMessage))

	msgList := starlark.NewList(nil)
	for _, msg := range p.AllUserMessages {
		msgList.Append(starlark.String(msg))
	}
	ctxDict.SetKey(starlark.String("all_user_messages"), msgList)
	ctxDict.SetKey(starlark.String("total_chars"), starlark.MakeInt(p.TotalChars))
	ctxDict.SetKey(starlark.String("approx_tokens"), starlark.MakeInt(p.ApproxTokens))
	ctxDict.SetKey(starlark.String("has_tool_calls"), starlark.Bool(p.HasToolCalls))
	ctxDict.SetKey(starlark.String("message_count"), starlark.MakeInt(p.MessageCount))

	modelDict := starlark.NewDict(len(p.ModelIDs))
	for k, v := range p.ModelIDs {
		modelDict.SetKey(starlark.String(k), starlark.String(v))
	}
	ctxDict.SetKey(starlark.String("model_ids"), modelDict)

	// log(msg) — no-op in Starlark, returns None.
	logFn := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	}
	ctxDict.SetKey(starlark.String("log"), starlark.NewBuiltin("log", logFn))

	// Convert payload to Starlark dict.
	payloadVal := payloadToStarlark(map[string]any{
		"model":    "",
		"messages": []any{},
	})

	thread := &starlark.Thread{
		Name: "rule:" + fn.Name(),
	}

	// Execute with timeout via goroutine + channel.
	type evalResult struct {
		val starlark.Value
		err error
	}
	resultCh := make(chan evalResult, 1)

	go func() {
		args := starlark.Tuple{payloadVal, ctxDict}
		val, err := starlark.Call(thread, fn, args, nil)
		resultCh <- evalResult{val, err}
	}()

	var result evalResult
	select {
	case result = <-resultCh:
	case <-time.After(starlarkTimeout):
		return Result{
			Match:  false,
			Target: "",
			Reason: fmt.Sprintf("Starlark rule %q timed out", fn.Name()),
		}
	}

	if result.err != nil {
		return Result{
			Match:  false,
			Target: "",
			Reason: fmt.Sprintf("Starlark rule %q error: %v", fn.Name(), result.err),
		}
	}

	// Expect result to be a dict with match, target, reason.
	resultDict, ok := result.val.(*starlark.Dict)
	if !ok {
		return Result{
			Match:  false,
			Target: "",
			Reason: fmt.Sprintf("Starlark rule %q: expected dict return, got %T", fn.Name(), result.val),
		}
	}

	match := false
	if matchVal, found, _ := resultDict.Get(starlark.String("match")); found {
		if b, ok := matchVal.(starlark.Bool); ok {
			match = bool(b)
		}
	}

	target := ""
	if targetVal, found, _ := resultDict.Get(starlark.String("target")); found {
		if s, ok := targetVal.(starlark.String); ok {
			target = string(s)
		}
	}

	reason := ""
	if reasonVal, found, _ := resultDict.Get(starlark.String("reason")); found {
		if s, ok := reasonVal.(starlark.String); ok {
			reason = string(s)
		}
	}

	if match && target != "" {
		return MatchResult(target, reason)
	}
	return NoMatch()
}

// payloadToStarlark converts a Go value to a Starlark value recursively.
func payloadToStarlark(v any) starlark.Value {
	switch x := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(x)
	case int:
		return starlark.MakeInt(x)
	case int64:
		return starlark.MakeInt64(x)
	case float64:
		return starlark.Float(x)
	case string:
		return starlark.String(x)
	case []any:
		items := make([]starlark.Value, len(x))
		for i, item := range x {
			items[i] = payloadToStarlark(item)
		}
		return starlark.NewList(items)
	case map[string]any:
		dict := starlark.NewDict(len(x))
		for k, v := range x {
			dict.SetKey(starlark.String(k), payloadToStarlark(v))
		}
		return dict
	default:
		return starlark.String(fmt.Sprintf("%v", x))
	}
}

// LoadStarlarkFiles scans a directory for .star files and returns compiled rules.
func LoadStarlarkFiles(dir string, cfg *config.Config) ([]Rule, error) {
	if dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var rules []Rule
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".star") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			if cfg.Routing.StrictValidation {
				return nil, fmt.Errorf("read Starlark file %s: %w", path, err)
			}
			continue
		}

		priority := ParsePriorityFromFilename(entry.Name())
		rule, err := NewStarlarkRule(entry.Name(), priority, string(data), cfg)
		if err != nil {
			if cfg.Routing.StrictValidation {
				return nil, fmt.Errorf("compile Starlark rule %s: %w", entry.Name(), err)
			}
			continue
		}
		rules = append(rules, rule)
	}

	return rules, nil
}
