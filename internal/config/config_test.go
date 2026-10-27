package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if len(cfg.Models) == 0 {
		t.Error("DefaultConfig() has no models")
	}
	if cfg.Upstream.BaseURL == "" {
		t.Error("DefaultConfig() has no upstream URL")
	}
	if cfg.Routing.StopOnFirstMatch != true {
		t.Error("DefaultConfig() should stop on first match")
	}
}

func TestLoadEmbedded(t *testing.T) {
	cfg, err := loadEmbedded()
	if err != nil {
		t.Fatalf("loadEmbedded() failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("loadEmbedded() returned nil")
	}
	// Embedded config should have the haiku model.
	if _, ok := cfg.Models["haiku"]; !ok {
		t.Error("embedded config missing haiku model")
	}
}

func TestLoadFromBytes(t *testing.T) {
	data := []byte(`
models:
  haiku:
    id: "claude-haiku-5-5"
    name: "Haiku 5.5"
  deepseek:
    id: "deepseek-v4-pro"
    name: "DeepSeek V4 Pro"
upstream:
  base_url: "https://test.example.com"
`)
	cfg, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes() failed: %v", err)
	}
	if cfg.Upstream.BaseURL != "https://test.example.com" {
		t.Errorf("expected upstream URL, got %q", cfg.Upstream.BaseURL)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
models:
  haiku:
    id: "claude-haiku-5-5"
upstream:
  base_url: "https://test.example.com"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Upstream.BaseURL != "https://test.example.com" {
		t.Errorf("expected upstream URL, got %q", cfg.Upstream.BaseURL)
	}
}

func TestLoadMissingFile(t *testing.T) {
	// Loading a non-existent path should fall back to embedded.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yaml")
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() with missing file should fall back to embedded: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil on fallback")
	}
	if _, ok := cfg.Models["haiku"]; !ok {
		t.Error("fallback config missing haiku model")
	}
}

func TestValidateDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if err := Validate(cfg); err != nil {
		t.Errorf("Validate(default config) should pass: %v", err)
	}
}

func TestValidateEmptyModels(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Models = map[string]ModelConfig{}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail with empty models")
	}
}

func TestValidateBadTarget(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Routing.Rules = []RuleDef{
		{Name: "bad", Type: "always", Target: "nonexistent"},
	}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail with bad target")
	}
}

func TestValidateBadPhraseSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Routing.Rules = []RuleDef{
		{
			Name:   "bad",
			Type:   "keyword_match",
			Target: "haiku",
			Params: map[string]any{"phrase_set": "nonexistent"},
		},
	}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail with bad phrase_set")
	}
}

func TestValidateMissingPhraseSetParam(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Routing.Rules = []RuleDef{
		{
			Name:   "bad",
			Type:   "keyword_match",
			Target: "haiku",
		},
	}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail when keyword_match has no params")
	}
}

func TestValidatePhraseSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Phrases = []PhraseSet{
		{Name: "", Phrases: []string{"hello"}},
	}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail with empty phrase set name")
	}
}

func TestValidatePhraseSetEmptyPhrases(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Phrases = []PhraseSet{
		{Name: "empty", Phrases: []string{}},
	}
	if err := Validate(cfg); err == nil {
		t.Error("Validate() should fail with empty phrases list")
	}
}

func TestFindConfigPath(t *testing.T) {
	// Should find config.yaml in current dir if it exists.
	path := FindConfigPath()
	_ = path // Just ensure no panic
}

func TestLoadBadYAML(t *testing.T) {
	data := []byte(`invalid yaml: [`)
	_, err := LoadFromBytes(data)
	if err == nil {
		t.Error("LoadFromBytes() should fail with bad YAML")
	}
}

func TestLoadRoundtripDefaultYAML(t *testing.T) {
	// Ensure that the embedded YAML parses cleanly.
	cfg, err := loadEmbedded()
	if err != nil {
		t.Fatalf("embedded YAML failed to parse: %v", err)
	}
	_ = cfg
}