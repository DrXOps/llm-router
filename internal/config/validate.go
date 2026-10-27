package config

import "fmt"

// Validate checks the configuration for consistency.
// Returns nil if valid, or an error describing all issues found.
func Validate(cfg *Config) error {
	var errs []string

	if len(cfg.Models) == 0 {
		errs = append(errs, "models: at least one model must be defined")
	}

	// Collect valid model keys for target reference checks.
	validModels := make(map[string]bool)
	for key := range cfg.Models {
		validModels[key] = true
	}

	// Build valid phrase set names.
	validPhraseSets := make(map[string]bool)
	for _, ps := range cfg.Phrases {
		if ps.Name == "" {
			errs = append(errs, "phrases: each phrase set must have a non-empty name")
			continue
		}
		if len(ps.Phrases) == 0 {
			errs = append(errs, fmt.Sprintf("phrases[%s]: must have at least one phrase", ps.Name))
		}
		validPhraseSets[ps.Name] = true
	}

	// Validate all rules (both built-in and those that will come from files).
	for i, r := range cfg.Routing.Rules {
		prefix := fmt.Sprintf("routing.rules[%d]", i)
		if r.Name == "" {
			errs = append(errs, prefix+": name is required")
		}
		if r.Type == "" {
			errs = append(errs, prefix+": type is required")
		}
		if r.Target != "" && !validModels[r.Target] {
			errs = append(errs, fmt.Sprintf("%s: target %q does not match any defined model", prefix, r.Target))
		}
		// Validate phrase_set reference if type is keyword_match.
		if r.Type == "keyword_match" {
			if r.Params == nil {
				errs = append(errs, fmt.Sprintf("%s: keyword_match requires params.phrase_set", prefix))
				continue
			}
			psName, _ := r.Params["phrase_set"].(string)
			if psName == "" || !validPhraseSets[psName] {
				errs = append(errs, fmt.Sprintf("%s: keyword_match references unknown phrase_set %q", prefix, psName))
			}
		}

		// Validate regex pattern for regex_match type.
		if r.Type == "regex_match" {
			if r.Params == nil {
				errs = append(errs, fmt.Sprintf("%s: regex_match requires params.pattern", prefix))
			}
		}

		// Validate threshold for context_length type.
		if r.Type == "context_length" {
			if r.Params == nil {
				errs = append(errs, fmt.Sprintf("%s: context_length requires params.threshold_tokens", prefix))
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return fmt.Errorf("validation errors:\n  - %s", joinStrings(errs, "\n  - "))
}

func joinStrings(items []string, sep string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += sep
		}
		result += item
	}
	return result
}