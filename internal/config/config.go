package config

// ModelConfig defines a single upstream model.
type ModelConfig struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}

// PhraseSet defines a named set of trigger phrases for keyword_match rules.
type PhraseSet struct {
	Name    string   `yaml:"name"`
	Phrases []string `yaml:"phrases"`
}

// RuleDef defines a single declarative rule (from config or YAML file).
type RuleDef struct {
	Name        string            `yaml:"name"`
	Priority    int               `yaml:"priority"`
	Enabled     bool              `yaml:"enabled"`
	Type        string            `yaml:"type"`
	Target      string            `yaml:"target"`
	Description string            `yaml:"description"`
	Params      map[string]any    `yaml:"params"`
}

// RoutingConfig controls rule loading and selection behaviour.
type RoutingConfig struct {
	RulesDir            string    `yaml:"rules_dir"`
	IncludeBuiltinRules bool      `yaml:"include_builtin_rules"`
	StopOnFirstMatch    bool      `yaml:"stop_on_first_match"`
	StrictValidation    bool      `yaml:"strict_validation"`
	WatchRules          bool      `yaml:"watch_rules"`
	EnableSLMClassifier bool      `yaml:"enable_slm_classifier"`
	SLMModelPath        string    `yaml:"slm_model_path"`
	SLMVocabPath        string    `yaml:"slm_vocab_path"`
	Rules               []RuleDef `yaml:"rules"`
}

// UpstreamConfig defines the target API endpoint.
type UpstreamConfig struct {
	BaseURL string `yaml:"base_url"`
}

// LoggingConfig controls debug output.
type LoggingConfig struct {
	Level              string `yaml:"level"`
	LogModelChoice     bool   `yaml:"log_model_choice"`
	LogRequestBody     bool   `yaml:"log_request_body"`
}

// Config is the top-level configuration structure.
type Config struct {
	Models   map[string]ModelConfig `yaml:"models"`
	Phrases  []PhraseSet            `yaml:"phrases"`
	Routing  RoutingConfig          `yaml:"routing"`
	Upstream UpstreamConfig         `yaml:"upstream"`
	Logging  LoggingConfig          `yaml:"logging"`
}

// DefaultConfig returns a ready-to-use default configuration.
func DefaultConfig() *Config {
	return &Config{
		Models: map[string]ModelConfig{
			"haiku":    {ID: "claude-haiku-5-5", Name: "Haiku 5.5"},
			"deepseek": {ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro"},
		},
		Phrases: []PhraseSet{
			{
				Name:    "complex_engineering",
				Phrases: []string{"refactor", "architecture", "refactoring"},
			},
		},
		Routing: RoutingConfig{
			RulesDir:            "./rules.d",
			IncludeBuiltinRules: true,
			StopOnFirstMatch:    true,
			StrictValidation:    false,
			WatchRules:          false,
			EnableSLMClassifier: false,
			SLMModelPath:        "model.bin",
			SLMVocabPath:        "vocab.txt",
			Rules: []RuleDef{
				{
					Name:        "always_haiku_fallback",
					Priority:    9999,
					Enabled:     true,
					Type:        "always",
					Target:      "haiku",
					Description: "Fallback: route to Haiku if no other rule matched",
				},
			},
		},
		Upstream: UpstreamConfig{
			BaseURL: "https://routerai.ru",
		},
		Logging: LoggingConfig{
			Level:              "info",
			LogModelChoice:     true,
			LogRequestBody:     false,
		},
	}
}