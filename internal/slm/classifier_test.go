package slm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewClassifierDisabled(t *testing.T) {
	c := NewClassifier(false, "", "")
	if c.Enabled() {
		t.Error("disabled classifier should not be enabled")
	}
	target, conf := c.Classify("hello")
	if target != "" {
		t.Errorf("expected empty target, got %q", target)
	}
	if conf != 0 {
		t.Errorf("expected 0 confidence, got %f", conf)
	}
}

func TestNewClassifierEnabledNoModel(t *testing.T) {
	c := NewClassifier(true, "/nonexistent/model.bin", "/nonexistent/vocab.txt")
	if c.Enabled() {
		t.Error("classifier without model files should gracefully degrade")
	}
}

func TestNewClassifierEnabledWithValidFiles(t *testing.T) {
	// We need actual model files for this. Without them, it degrades.
	c := NewClassifier(true, "testdata/model.bin", "testdata/vocab.txt")
	if c.Enabled() {
		t.Log("model files exist, classifier is enabled")
	} else {
		t.Log("model files not found, classifier gracefully degraded (expected if no testdata)")
	}
}

func TestClassifierDebugInfo(t *testing.T) {
	c := NewClassifier(false, "", "")
	info := c.DebugInfo()
	if info["enabled"] != false {
		t.Error("disabled classifier should show enabled=false")
	}
}

func TestTruncateStr(t *testing.T) {
	if truncateStr("hello", 10) != "hello" {
		t.Error("should not truncate short strings")
	}
	if truncateStr("hello world", 5) != "hello..." {
		t.Error("should truncate and add ellipsis")
	}
}

func TestTokenizerBasic(t *testing.T) {
	// Without a real vocab file, we can only test with a minimal one.
	dir := t.TempDir()
	vocabPath := filepath.Join(dir, "vocab.txt")
	vocab := "[CLS]\n[SEP]\n[UNK]\n[PAD]\nthe\nhello\nworld\n##ing\n"
	os.WriteFile(vocabPath, []byte(vocab), 0644)

	tok, err := NewTokenizerFromFile(vocabPath)
	if err != nil {
		t.Fatalf("NewTokenizerFromFile failed: %v", err)
	}
	if tok.VocabSize() != 8 {
		t.Errorf("expected vocab size 8, got %d", tok.VocabSize())
	}

	ids := tok.Encode("hello world")
	if len(ids) < 3 {
		t.Errorf("expected at least 3 tokens ([CLS] hello world [SEP]), got %d", len(ids))
	}
	// Should start with [CLS] (0) and end with [SEP] (1)
	if ids[0] != 0 {
		t.Errorf("expected first token [CLS](0), got %d", ids[0])
	}
}

func TestLogisticHeadPredict(t *testing.T) {
	head := &LogisticHead{
		Weight: []float32{1.0, 0.0, 0.0, 1.0}, // 2 classes × 2 features
		Bias:   []float32{0.0, 0.0},
		Labels: []string{"haiku", "deepseek"},
	}

	embedding := []float32{1.0, 0.0}
	target, conf, scores := head.Predict(embedding)
	if target != "haiku" {
		t.Errorf("expected haiku for [1,0], got %q", target)
	}
	if conf < 0.5 {
		t.Errorf("expected high confidence, got %f", conf)
	}
	if scores["haiku"] < 0.5 {
		t.Errorf("expected haiku score > 0.5, got %f", scores["haiku"])
	}
}

func TestLogisticHeadPredictDeepSeek(t *testing.T) {
	head := &LogisticHead{
		Weight: []float32{1.0, 0.0, 0.0, 1.0},
		Bias:   []float32{0.0, 0.0},
		Labels: []string{"haiku", "deepseek"},
	}

	embedding := []float32{0.0, 1.0}
	target, _, _ := head.Predict(embedding)
	if target != "deepseek" {
		t.Errorf("expected deepseek for [0,1], got %q", target)
	}
}

func TestArgmax(t *testing.T) {
	if argmax([]float32{1, 3, 2}) != 1 {
		t.Error("argmax should return index of largest value")
	}
	if argmax([]float32{}) != -1 {
		t.Error("argmax of empty slice should return -1")
	}
}

func TestOpsAddVectors(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{10, 20, 30}
	addVectors(a, b)
	if a[0] != 11 || a[1] != 22 || a[2] != 33 {
		t.Errorf("addVectors: expected [11 22 33], got %v", a)
	}
}

func TestWeightLoadingNonExistent(t *testing.T) {
	_, err := LoadMiniLMBin("/nonexistent/model.bin")
	if err == nil {
		t.Error("expected error loading non-existent file")
	}
}