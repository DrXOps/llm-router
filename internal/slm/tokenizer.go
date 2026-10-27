package slm

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// Tokenizer implements WordPiece tokenization for MiniLM models.
type Tokenizer struct {
	vocab    map[string]int32
	ids      map[int32]string
	clsToken int32
	sepToken int32
	unkToken int32
	maxLen   int
}

// NewTokenizerFromFile loads a WordPiece vocabulary from a file.
func NewTokenizerFromFile(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int32)
	ids := make(map[int32]string)
	scanner := bufio.NewScanner(f)
	var idx int32
	for scanner.Scan() {
		token := scanner.Text()
		vocab[token] = idx
		ids[idx] = token
		idx++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Tokenizer{
		vocab:    vocab,
		ids:      ids,
		clsToken: vocab["[CLS]"],
		sepToken: vocab["[SEP]"],
		unkToken: vocab["[UNK]"],
		maxLen:   256,
	}, nil
}

// Encode converts text to token IDs with WordPiece.
func (t *Tokenizer) Encode(text string) []int32 {
	tokens := basicTokenize(text)
	var wpTokens []string
	for _, token := range tokens {
		wp := t.wordpiece(token)
		wpTokens = append(wpTokens, wp...)
	}

	// Truncate
	if len(wpTokens) > t.maxLen-2 {
		wpTokens = wpTokens[:t.maxLen-2]
	}

	ids := make([]int32, 0, len(wpTokens)+2)
	ids = append(ids, t.clsToken)
	for _, w := range wpTokens {
		if id, ok := t.vocab[w]; ok {
			ids = append(ids, id)
		} else {
			ids = append(ids, t.unkToken)
		}
	}
	ids = append(ids, t.sepToken)

	return ids
}

// Decode converts token IDs back to a readable string (for debugging).
func (t *Tokenizer) Decode(ids []int32) string {
	var parts []string
	for _, id := range ids {
		if token, ok := t.ids[id]; ok {
			parts = append(parts, token)
		}
	}
	return strings.Join(parts, " ")
}

// basicTokenize splits text on whitespace and punctuation.
func basicTokenize(text string) []string {
	var tokens []string
	current := make([]rune, 0)
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = current[:0]
			}
			if unicode.IsPunct(r) {
				tokens = append(tokens, string(r))
			}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}
	return tokens
}

// wordpiece splits a token into WordPiece sub-tokens.
func (t *Tokenizer) wordpiece(token string) []string {
	if _, ok := t.vocab[token]; ok {
		return []string{token}
	}

	var pieces []string
	runes := []rune(token)
	start := 0
	for start < len(runes) {
		end := len(runes)
		found := false
		for end > start {
			sub := string(runes[start:end])
			if start > 0 {
				sub = "##" + sub
			}
			if _, ok := t.vocab[sub]; ok {
				pieces = append(pieces, sub)
				found = true
				break
			}
			end--
		}
		if !found {
			pieces = append(pieces, "[UNK]")
			break
		}
		start = end
	}
	return pieces
}

// VocabSize returns the vocabulary size.
func (t *Tokenizer) VocabSize() int {
	return len(t.vocab)
}

// PadToken returns the padding token ID (0).
func (t *Tokenizer) PadToken() int32 { return 0 }

// CLSToken returns the [CLS] token ID.
func (t *Tokenizer) CLSToken() int32 { return t.clsToken }

// SEPToken returns the [SEP] token ID.
func (t *Tokenizer) SEPToken() int32 { return t.sepToken }

// MaxLen returns the max sequence length.
func (t *Tokenizer) MaxLen() int { return t.maxLen }