package slm

import "math"

// MiniLMConfig holds the hyperparameters of the model.
type MiniLMConfig struct {
	VocabSize     int
	HiddenSize    int
	NumLayers     int
	NumHeads      int
	Intermediate  int
	MaxSeqLen     int
	LayerNormEps  float32
}

// DefaultMiniLMConfig returns the config for multilingual-MiniLM-L6-v2.
func DefaultMiniLMConfig() MiniLMConfig {
	return MiniLMConfig{
		VocabSize:    250000, // will be overridden by tokenizer
		HiddenSize:   384,
		NumLayers:    6,
		NumHeads:     6,
		Intermediate: 1536,
		MaxSeqLen:    256,
		LayerNormEps: 1e-12,
	}
}

// === Embedding ===

// embeddingLookup looks up token IDs in the weight matrix [vocab_size × hidden].
func embeddingLookup(ids []int32, weight []float32, hidden int) []float32 {
	out := make([]float32, len(ids)*hidden)
	for i, id := range ids {
		if int(id) < 0 {
			continue
		}
		src := weight[int(id)*hidden : (int(id)+1)*hidden]
		dst := out[i*hidden : (i+1)*hidden]
		copy(dst, src)
	}
	return out
}

// addPositionalEmbedding adds learned positional embeddings [max_seq_len × hidden].
func addPositionalEmbedding(hidden []float32, posWeight []float32, seqLen, hiddenSize int) {
	for i := 0; i < seqLen; i++ {
		start := i * hiddenSize
		posStart := i * hiddenSize
		for j := 0; j < hiddenSize; j++ {
			hidden[start+j] += posWeight[posStart+j]
		}
	}
}

// === Attention ===

// attention computes multi-head self-attention on a single batch (seq_len × hidden).
// Returns attended output [seq_len × hidden].
func attention(qkv [][3][]float32, seqLen, hiddenSize, numHeads int, attnWeight, attnBias, outWeight, outBias []float32) []float32 {
	headDim := hiddenSize / numHeads
	// QKV projection: all heads x [q|k|v] in one go.
	// Weight shape: [hidden × 3*hidden]
	qkvProj := make([]float32, seqLen*3*hiddenSize)
	matMul(qkv[0][0], attnWeight[:hiddenSize*3*hiddenSize], qkvProj, seqLen, hiddenSize, 3*hiddenSize)
	addVectors(qkvProj, attnBias)

	// Split into Q, K, V [seq_len × num_heads × head_dim]
	q := make([]float32, seqLen*numHeads*headDim)
	k := make([]float32, seqLen*numHeads*headDim)
	v := make([]float32, seqLen*numHeads*headDim)

	for i := 0; i < seqLen; i++ {
		row := qkvProj[i*3*hiddenSize : (i+1)*3*hiddenSize]
		copy(q[i*numHeads*headDim:(i+1)*numHeads*headDim], row[:numHeads*headDim])
		copy(k[i*numHeads*headDim:(i+1)*numHeads*headDim], row[numHeads*headDim:2*numHeads*headDim])
		copy(v[i*numHeads*headDim:(i+1)*numHeads*headDim], row[2*numHeads*headDim:3*numHeads*headDim])
	}

	// Reshape to [num_heads × seq_len × head_dim] by transposing.
	qHeads := make([]float32, numHeads*seqLen*headDim)
	kHeads := make([]float32, numHeads*seqLen*headDim)
	vHeads := make([]float32, numHeads*seqLen*headDim)

	for h := 0; h < numHeads; h++ {
		for s := 0; s < seqLen; s++ {
			srcQ := q[s*numHeads*headDim+h*headDim : s*numHeads*headDim+(h+1)*headDim]
			srcK := k[s*numHeads*headDim+h*headDim : s*numHeads*headDim+(h+1)*headDim]
			srcV := v[s*numHeads*headDim+h*headDim : s*numHeads*headDim+(h+1)*headDim]
			dstQ := qHeads[h*seqLen*headDim+s*headDim : h*seqLen*headDim+(s+1)*headDim]
			dstK := kHeads[h*seqLen*headDim+s*headDim : h*seqLen*headDim+(s+1)*headDim]
			dstV := vHeads[h*seqLen*headDim+s*headDim : h*seqLen*headDim+(s+1)*headDim]
			copy(dstQ, srcQ)
			copy(dstK, srcK)
			copy(dstV, srcV)
		}
	}

	// Compute attention scores: Q @ K.T for each head.
	// Q_heads[h]: [seq_len × head_dim], K_heads[h]: [seq_len × head_dim]
	// scores: [num_heads × seq_len × seq_len]
	scores := make([]float32, numHeads*seqLen*seqLen)
	for h := 0; h < numHeads; h++ {
		qHead := qHeads[h*seqLen*headDim : (h+1)*seqLen*headDim]
		kHead := kHeads[h*seqLen*headDim : (h+1)*seqLen*headDim]
		score := scores[h*seqLen*seqLen : (h+1)*seqLen*seqLen]
		// kHead needs to be transposed: [seq_len × head_dim] → [head_dim × seq_len]
		kHeadT := make([]float32, headDim*seqLen)
		transpose(kHead, kHeadT, seqLen, headDim)
		matMul(qHead, kHeadT, score, seqLen, headDim, seqLen)
		// Scale by 1/sqrt(head_dim)
		scale(score, float32(1.0/math.Sqrt(float64(headDim))))
	}

	// Softmax over last dimension (seq_len).
	softmax(scores, seqLen)

	// Apply attention to V: scores @ V for each head.
	// output: [num_heads × seq_len × head_dim]
	attended := make([]float32, numHeads*seqLen*headDim)
	for h := 0; h < numHeads; h++ {
		score := scores[h*seqLen*seqLen : (h+1)*seqLen*seqLen]
		vHead := vHeads[h*seqLen*headDim : (h+1)*seqLen*headDim]
		out := attended[h*seqLen*headDim : (h+1)*seqLen*headDim]
		matMul(score, vHead, out, seqLen, seqLen, headDim)
	}

	// Transpose back to [seq_len × num_heads × head_dim].
	attendedSeq := make([]float32, seqLen*numHeads*headDim)
	for s := 0; s < seqLen; s++ {
		for h := 0; h < numHeads; h++ {
			src := attended[h*seqLen*headDim+s*headDim : h*seqLen*headDim+(s+1)*headDim]
			dst := attendedSeq[s*numHeads*headDim+h*headDim : s*numHeads*headDim+(h+1)*headDim]
			copy(dst, src)
		}
	}

	// Output projection: [seq_len × hidden].
	out := make([]float32, seqLen*hiddenSize)
	matMul(attendedSeq, outWeight, out, seqLen, hiddenSize, hiddenSize)
	addVectors(out, outBias)

	return out
}

// === Feed-Forward Network ===

// ffn applies: GELU(x @ W1 + b1) @ W2 + b2
func ffn(hidden []float32, seqLen, hiddenSize, intermediate int, w1, b1, w2, b2 []float32) []float32 {
	inter := make([]float32, seqLen*intermediate)
	matMul(hidden, w1, inter, seqLen, hiddenSize, intermediate)
	addVectors(inter, b1)
	gelu(inter)

	out := make([]float32, seqLen*hiddenSize)
	matMul(inter, w2, out, seqLen, intermediate, hiddenSize)
	addVectors(out, b2)
	return out
}

// === Transformer Layer ===

// transformerLayer runs one layer of MiniLM.
// Input: [seq_len × hidden], Output: [seq_len × hidden].
func transformerLayer(input []float32, seqLen int, cfg MiniLMConfig,
	attnQKVWeight, attnQKVBias, attnOutWeight, attnOutBias []float32,
	ln1Weight, ln1Bias []float32,
	ffnW1, ffnB1, ffnW2, ffnB2 []float32,
	ln2Weight, ln2Bias []float32) []float32 {

	hiddenSize := cfg.HiddenSize

	// Pre-LN: LayerNorm → Attention → Residual
	normed := make([]float32, len(input))
	copy(normed, input)
	layerNorm(normed, ln1Weight, ln1Bias, cfg.LayerNormEps)

	// Placeholder for QKV (we already have attention that takes weights)
	attended := attention(nil, seqLen, hiddenSize, cfg.NumHeads, attnQKVWeight, attnQKVBias, attnOutWeight, attnOutBias)

	// Residual
	for i := range input {
		attended[i] += input[i]
	}

	// Pre-LN: LayerNorm → FFN → Residual
	normed2 := make([]float32, len(attended))
	copy(normed2, attended)
	layerNorm(normed2, ln2Weight, ln2Bias, cfg.LayerNormEps)

	ffnOut := ffn(normed2, seqLen, hiddenSize, cfg.Intermediate, ffnW1, ffnB1, ffnW2, ffnB2)

	// Residual
	for i := range attended {
		ffnOut[i] += attended[i]
	}

	return ffnOut
}

// === Full MiniLM Model ===

// MiniLM holds all weights and runs the forward pass.
type MiniLM struct {
	Config MiniLMConfig

	// Embeddings
	TokenEmbeddings []float32 // [vocab_size × hidden]
	PosEmbeddings   []float32 // [max_seq_len × hidden]
	EmbedLNWeight   []float32 // [hidden]
	EmbedLNBias     []float32 // [hidden]

	// Per-layer weights
	LayerAttnQKVW [][]float32 // [num_layers][hidden × 3*hidden]
	LayerAttnQKVB [][]float32 // [num_layers][3*hidden]
	LayerAttnOutW [][]float32 // [num_layers][hidden × hidden]
	LayerAttnOutB [][]float32 // [num_layers][hidden]
	LayerLN1W     [][]float32 // [num_layers][hidden]
	LayerLN1B     [][]float32 // [num_layers][hidden]
	LayerFFNW1    [][]float32 // [num_layers][hidden × intermediate]
	LayerFFNB1    [][]float32 // [num_layers][intermediate]
	LayerFFNW2    [][]float32 // [num_layers][intermediate × hidden]
	LayerFFNB2    [][]float32 // [num_layers][hidden]
	LayerLN2W     [][]float32 // [num_layers][hidden]
	LayerLN2B     [][]float32 // [num_layers][hidden]

	// Pooler (for sentence embeddings)
	PoolerWeight []float32 // [hidden × hidden]
	PoolerBias   []float32 // [hidden]
}

// Forward runs the full MiniLM model and returns a pooled embedding [hidden_size].
func (m *MiniLM) Forward(inputIDs []int32) []float32 {
	seqLen := len(inputIDs)
	hs := m.Config.HiddenSize

	// Token embeddings.
	hidden := embeddingLookup(inputIDs, m.TokenEmbeddings, hs)

	// Add positional embeddings.
	addPositionalEmbedding(hidden, m.PosEmbeddings, seqLen, hs)

	// Embedding LayerNorm.
	for i := 0; i < seqLen; i++ {
		row := hidden[i*hs : (i+1)*hs]
		layerNorm(row, m.EmbedLNWeight, m.EmbedLNBias, m.Config.LayerNormEps)
	}

	// Transformer layers.
	for l := 0; l < m.Config.NumLayers; l++ {
		hidden = transformerLayer(hidden, seqLen, m.Config,
			m.LayerAttnQKVW[l], m.LayerAttnQKVB[l],
			m.LayerAttnOutW[l], m.LayerAttnOutB[l],
			m.LayerLN1W[l], m.LayerLN1B[l],
			m.LayerFFNW1[l], m.LayerFFNB1[l],
			m.LayerFFNW2[l], m.LayerFFNB2[l],
			m.LayerLN2W[l], m.LayerLN2B[l],
		)
	}

	// Mean pooling over sequence.
	pooled := meanPool(hidden, seqLen, hs)

	// Pooler: tanh(pooled @ W + b)
	pooledOut := make([]float32, hs)
	matMul(pooled, m.PoolerWeight, pooledOut, 1, hs, hs)
	addVectors(pooledOut, m.PoolerBias)
	for i := range pooledOut {
		pooledOut[i] = float32(math.Tanh(float64(pooledOut[i])))
	}

	return pooledOut
}