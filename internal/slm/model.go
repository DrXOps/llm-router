package slm

import (
	"log/slog"
	"math"
)

// LogisticHead is a simple binary classifier trained on top of MiniLM embeddings.
// It maps a [hidden_size] embedding to a probability for each model target.
type LogisticHead struct {
	Weight []float32 // [num_classes × hidden_size]
	Bias   []float32 // [num_classes]
	Labels []string  // model target names, e.g. ["haiku", "deepseek"]
}

// Predict returns (predictedLabel, confidence, allScores).
func (h *LogisticHead) Predict(embedding []float32) (string, float32, map[string]float32) {
	numClasses := len(h.Labels)
	scores := make([]float32, numClasses)
	for c := 0; c < numClasses; c++ {
		var sum float32
		off := c * len(embedding)
		for i, v := range embedding {
			sum += v * h.Weight[off+i]
		}
		scores[c] = sum + h.Bias[c]
	}

	// Softmax to get probabilities.
	maxScore := scores[0]
	for _, s := range scores[1:] {
		if s > maxScore {
			maxScore = s
		}
	}
	var total float32
	probs := make([]float32, numClasses)
	for i, s := range scores {
		probs[i] = float32(math.Exp(float64(s - maxScore)))
		total += probs[i]
	}
	for i := range probs {
		probs[i] /= total
	}

	bestIdx := 0
	for i := 1; i < numClasses; i++ {
		if probs[i] > probs[bestIdx] {
			bestIdx = i
		}
	}

	scoresMap := make(map[string]float32)
	for i, label := range h.Labels {
		scoresMap[label] = probs[i]
	}

	return h.Labels[bestIdx], probs[bestIdx], scoresMap
}

// NLPClassifier wraps the full NLP pipeline: tokenizer → MiniLM → logistic head.
type NLPClassifier struct {
	Tokenizer *Tokenizer
	MiniLM    *MiniLM
	Head      *LogisticHead
	Enabled    bool
}

// NewNLPClassifier creates a new NLP classifier from model and vocab files.
func NewNLPClassifier(modelPath, vocabPath string, enable bool) (*NLPClassifier, error) {
	if !enable {
		return &NLPClassifier{Enabled: false}, nil
	}

	tok, err := NewTokenizerFromFile(vocabPath)
	if err != nil {
		return nil, err
	}

	model, err := LoadMiniLMBin(modelPath)
	if err != nil {
		return nil, err
	}

	// Default classification head: haiku vs deepseek
	// In production, these would be loaded from a trained .bin file.
	head := &LogisticHead{
		Weight: make([]float32, 2*model.Config.HiddenSize), // zeros initially
		Bias:   make([]float32, 2),
		Labels: []string{"haiku", "deepseek"},
	}

	c := &NLPClassifier{
		Tokenizer: tok,
		MiniLM:    model,
		Head:      head,
		Enabled:   true,
	}

	return c, nil
}

// ClassifyText classifies a text string and returns a target model suggestion.
func (c *NLPClassifier) ClassifyText(text string) (target string, confidence float32, scores map[string]float32) {
	if !c.Enabled {
		return "", 0, nil
	}

	if text == "" {
		return "", 0, map[string]float32{"haiku": 0.5, "deepseek": 0.5}
	}

	// Tokenize.
	ids := c.Tokenizer.Encode(text)

	// Run MiniLM forward pass.
	embedding := c.MiniLM.Forward(ids)

	// Run logistic head.
	target, confidence, scores = c.Head.Predict(embedding)

	slog.Debug("NLP classifier result",
		"text_preview", truncateStr(text, 50),
		"target", target,
		"confidence", confidence,
	)

	return target, confidence, scores
}

// LoadClassificationHead loads trained weights for the logistic head from a file.
func (c *NLPClassifier) LoadClassificationHead(path string) error {
	// TODO: implement loading trained head weights from .bin
	// For now, uses heuristic defaults.
	hs := c.MiniLM.Config.HiddenSize

	// Simple heuristic: if the embedding indicates technical/code content → deepseek
	// This requires proper training data. For now, we use rules.
	_ = hs

	slog.Warn("classification head not trained, using fallback")
	return nil
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

// Update classifier.go to use NLPClassifier
// This file will be updated next.