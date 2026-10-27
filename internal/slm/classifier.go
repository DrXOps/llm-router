package slm

import (
	"log/slog"
)

// Classifier is the main routing classifier.
// It uses NLP (MiniLM) when enabled and available, falling back to no-op.
type Classifier struct {
	nlp    *NLPClassifier
	enabled bool
}

// NewClassifier creates a classifier. When enable is true and model/vocab files
// are found, it loads the full NLP pipeline. Otherwise gracefully degrades.
func NewClassifier(enable bool, modelPath, vocabPath string) *Classifier {
	if !enable {
		slog.Debug("SLM classifier disabled")
		return &Classifier{enabled: false}
	}

	nlp, err := NewNLPClassifier(modelPath, vocabPath, true)
	if err != nil {
		slog.Warn("SLM NLP classifier not available, falling back to rules",
			"error", err,
			"model_path", modelPath,
		)
		return &Classifier{enabled: false}
	}

	if !nlp.Enabled {
		slog.Warn("SLM NLP classifier initialized but not enabled")
		return &Classifier{enabled: false}
	}

	slog.Info("SLM NLP classifier loaded",
		"model", "multilingual-MiniLM-L6",
		"embedding_size", nlp.MiniLM.Config.HiddenSize,
	)
	return &Classifier{
		nlp:     nlp,
		enabled: true,
	}
}

// Classify returns the suggested model target and confidence based on the
// user's last message text. Returns empty string if classifier is not enabled.
func (c *Classifier) Classify(lastUserMessage string) (target string, confidence float32) {
	if !c.enabled || c.nlp == nil {
		return "", 0
	}

	target, confidence, _ = c.nlp.ClassifyText(lastUserMessage)
	return target, confidence
}

// Enabled returns whether the classifier is active.
func (c *Classifier) Enabled() bool {
	return c.enabled
}

// DebugInfo returns information about the classifier state.
func (c *Classifier) DebugInfo() map[string]any {
	if !c.enabled {
		return map[string]any{"enabled": false}
	}
	return map[string]any{
		"enabled":  true,
		"model":    "multilingual-MiniLM-L6",
		"hidden":   c.nlp.MiniLM.Config.HiddenSize,
		"layers":   c.nlp.MiniLM.Config.NumLayers,
		"vocab":    c.nlp.Tokenizer.VocabSize(),
	}
}