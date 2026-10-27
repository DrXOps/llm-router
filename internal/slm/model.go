package slm

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
)

// LogisticHead is a binary classifier trained on top of MiniLM embeddings.
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

// LoadClassificationHead loads trained weights from a .bin file.
func LoadClassificationHead(path string, hiddenSize int) (*LogisticHead, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read header: num_classes, hidden_size.
	var numClasses, hs int32
	if err := binary.Read(f, binary.LittleEndian, &numClasses); err != nil {
		return nil, fmt.Errorf("read num_classes: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, &hs); err != nil {
		return nil, fmt.Errorf("read hidden_size: %w", err)
	}
	if int(hs) != hiddenSize {
		return nil, fmt.Errorf("hidden_size mismatch: head=%d, model=%d", hs, hiddenSize)
	}

	// Read labels.
	labels := make([]string, numClasses)
	for i := 0; i < int(numClasses); i++ {
		var labelLen int32
		if err := binary.Read(f, binary.LittleEndian, &labelLen); err != nil {
			return nil, fmt.Errorf("read label %d length: %w", i, err)
		}
		labelBytes := make([]byte, labelLen)
		if _, err := io.ReadFull(f, labelBytes); err != nil {
			return nil, fmt.Errorf("read label %d bytes: %w", i, err)
		}
		labels[i] = string(labelBytes[:len(labelBytes)-1]) // strip null terminator
	}

	// Read weights.
	weight := make([]float32, numClasses*hs)
	if err := binary.Read(f, binary.LittleEndian, &weight); err != nil {
		return nil, fmt.Errorf("read weights: %w", err)
	}

	// Read bias.
	bias := make([]float32, numClasses)
	if err := binary.Read(f, binary.LittleEndian, &bias); err != nil {
		return nil, fmt.Errorf("read bias: %w", err)
	}

	return &LogisticHead{
		Weight: weight,
		Bias:   bias,
		Labels: labels,
	}, nil
}

// NLPClassifier wraps the full NLP pipeline: tokenizer → MiniLM → logistic head.
type NLPClassifier struct {
	Tokenizer *Tokenizer
	MiniLM    *MiniLM
	Head      *LogisticHead
	Enabled   bool
}

// NewNLPClassifier creates a new NLP classifier from model, vocab, and head files.
func NewNLPClassifier(modelPath, vocabPath, headPath string, enable bool) (*NLPClassifier, error) {
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

	// Try to load trained classification head.
	head, err := LoadClassificationHead(headPath, model.Config.HiddenSize)
	if err != nil {
		slog.Warn("classification head not loaded, using untrained defaults", "error", err)
		head = &LogisticHead{
			Weight: make([]float32, 2*model.Config.HiddenSize),
			Bias:   make([]float32, 2),
			Labels: []string{"haiku", "deepseek"},
		}
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

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}