package router

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/user/llm-router/internal/config"
)

func TestScanDirModTimes(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("name: test\n"), 0644)
	os.WriteFile(filepath.Join(dir, "test.star"), []byte("def rule(p, c): pass\n"), 0644)
	os.WriteFile(filepath.Join(dir, "note.txt"), []byte("not a rule"), 0644)

	modTimes := make(map[string]time.Time)
	err := scanDirModTimes(dir, modTimes)
	if err != nil {
		t.Fatalf("scanDirModTimes failed: %v", err)
	}
	if len(modTimes) != 2 {
		t.Errorf("expected 2 rule files, got %d", len(modTimes))
	}
}

func TestScanDirModTimesNonExistent(t *testing.T) {
	modTimes := make(map[string]time.Time)
	err := scanDirModTimes("/nonexistent", modTimes)
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestWatchRulesDisabled(t *testing.T) {
	cfg := &config.Config{Routing: config.RoutingConfig{WatchRules: false}}
	ch := WatchRules(cfg, 5*time.Second)
	// Should be closed immediately.
	_, ok := <-ch
	if ok {
		t.Error("expected closed channel for disabled watcher")
	}
}