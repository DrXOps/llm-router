package slm

import (
	"log/slog"
)

// Classifier is an optional local ONNX-based model classifier.
// If the ONNX model cannot be loaded, it gracefully degrades to no-op.
type Classifier struct {
	enabled bool
}

// NewClassifier creates an optional classifier. If enable is false or the model
// cannot be loaded, returns a no-op classifier with enabled=false.
func NewClassifier(enable bool, modelPath string) *Classifier {
	if !enable {
		slog.Debug("SLM classifier disabled")
		return &Classifier{enabled: false}
	}

	if modelPath == "" {
		slog.Warn("SLM classifier enabled but no model path specified, falling back")
		return &Classifier{enabled: false}
	}

	// In a full implementation, this would load an ONNX model via gomlx/onnx-gomlx.
	// For now, we provide the graceful degradation stub.
	slog.Warn("SLM ONNX classifier not yet linked (gomlx requires CGO workaround); "+
		"falling back to rule-based routing", "model_path", modelPath)
	return &Classifier{enabled: false}
}

// Classify returns a model target suggestion ("haiku", "deepseek") or empty string
// if the classifier has no opinion.
func (c *Classifier) Classify(totalChars, messageCount int, hasToolCalls bool) string {
	if !c.enabled {
		return ""
	}
	// Stub: when enabled and loaded, would return a model suggestion.
	return ""
}

// Enabled returns whether the classifier is active.
func (c *Classifier) Enabled() bool {
	return c.enabled
}