package slm

import "testing"

func TestNewClassifierDisabled(t *testing.T) {
	c := NewClassifier(false, "")
	if c.Enabled() {
		t.Error("disabled classifier should not be enabled")
	}
	if c.Classify(100, 5, false) != "" {
		t.Error("disabled classifier should return empty")
	}
}

func TestNewClassifierEnabledNoModel(t *testing.T) {
	c := NewClassifier(true, "")
	if c.Enabled() {
		t.Error("classifier without model should not be enabled")
	}
}

func TestNewClassifierEnabledWithModel(t *testing.T) {
	// ONNX model loading is not implemented; this should gracefully degrade.
	c := NewClassifier(true, "/nonexistent/model.onnx")
	if c.Enabled() {
		t.Error("classifier with nonexistent model should gracefully degrade")
	}
}