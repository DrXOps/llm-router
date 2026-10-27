package slm

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// LoadMiniLMWeights loads a .bin file with all MiniLM weights.
// Binary format: [num_layers: int32][layer1_weights]...[layerN_weights][embeddings][pooler]
// Each weight is: [name_len: int32][name: name_len bytes][dims: 2×int32][data: float32[prod(dims)]]
// Simplified format: just flat binary, layer by layer, all dimensions known from config.
//
// For our custom format (simpler): all float32 values concatenated in a known order.
// File structure (all little-endian):
//   1) header: vocab_size(int32), hidden_size(int32), num_layers(int32), num_heads(int32), intermediate(int32), max_seq_len(int32)
//   2) token_embeddings:   vocab_size × hidden_size    float32s
//   3) pos_embeddings:     max_seq_len × hidden_size    float32s
//   4) embed_ln_weight:    hidden_size                  float32s
//   5) embed_ln_bias:      hidden_size                  float32s
//   6) for each layer l in 0..num_layers:
//      a) attn_qkv_weight: hidden_size × 3*hidden_size float32s
//      b) attn_qkv_bias:   3*hidden_size               float32s
//      c) attn_out_weight: hidden_size × hidden_size    float32s
//      d) attn_out_bias:   hidden_size                  float32s
//      e) ln1_weight:      hidden_size                  float32s
//      f) ln1_bias:        hidden_size                  float32s
//      g) ffn_w1:          hidden_size × intermediate   float32s
//      h) ffn_b1:          intermediate                 float32s
//      i) ffn_w2:          intermediate × hidden_size   float32s
//      j) ffn_b2:          hidden_size                  float32s
//      k) ln2_weight:      hidden_size                  float32s
//      l) ln2_bias:        hidden_size                  float32s
//   7) pooler_weight:      hidden_size × hidden_size    float32s
//   8) pooler_bias:        hidden_size                  float32s

func loadFloat32s(r io.Reader, n int) ([]float32, error) {
	data := make([]float32, n)
	if err := binary.Read(r, binary.LittleEndian, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func loadInt32(r io.Reader) (int32, error) {
	var v int32
	err := binary.Read(r, binary.LittleEndian, &v)
	return v, err
}

// LoadMiniLMBin loads a MiniLM model from a .bin file.
func LoadMiniLMBin(path string) (*MiniLM, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open model file: %w", err)
	}
	defer f.Close()

	// Read header.
	vocabSize, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read vocab_size: %w", err)
	}
	hiddenSize, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read hidden_size: %w", err)
	}
	numLayers, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read num_layers: %w", err)
	}
	numHeads, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read num_heads: %w", err)
	}
	intermediate, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read intermediate: %w", err)
	}
	maxSeqLen, err := loadInt32(f)
	if err != nil {
		return nil, fmt.Errorf("read max_seq_len: %w", err)
	}

	m := &MiniLM{
		Config: MiniLMConfig{
			VocabSize:    int(vocabSize),
			HiddenSize:   int(hiddenSize),
			NumLayers:    int(numLayers),
			NumHeads:     int(numHeads),
			Intermediate: int(intermediate),
			MaxSeqLen:    int(maxSeqLen),
			LayerNormEps: 1e-12,
		},
	}

	vs := m.Config.VocabSize
	hs := m.Config.HiddenSize
	nl := m.Config.NumLayers
	inter := m.Config.Intermediate
	msl := m.Config.MaxSeqLen

	// Token embeddings.
	m.TokenEmbeddings, err = loadFloat32s(f, vs*hs)
	if err != nil {
		return nil, fmt.Errorf("read token embeddings: %w", err)
	}

	// Positional embeddings.
	m.PosEmbeddings, err = loadFloat32s(f, msl*hs)
	if err != nil {
		return nil, fmt.Errorf("read pos embeddings: %w", err)
	}

	// Embedding LN.
	m.EmbedLNWeight, err = loadFloat32s(f, hs)
	if err != nil {
		return nil, fmt.Errorf("read embed_ln_weight: %w", err)
	}
	m.EmbedLNBias, err = loadFloat32s(f, hs)
	if err != nil {
		return nil, fmt.Errorf("read embed_ln_bias: %w", err)
	}

	// Per-layer weights.
	m.LayerAttnQKVW = make([][]float32, nl)
	m.LayerAttnQKVB = make([][]float32, nl)
	m.LayerAttnOutW = make([][]float32, nl)
	m.LayerAttnOutB = make([][]float32, nl)
	m.LayerLN1W = make([][]float32, nl)
	m.LayerLN1B = make([][]float32, nl)
	m.LayerFFNW1 = make([][]float32, nl)
	m.LayerFFNB1 = make([][]float32, nl)
	m.LayerFFNW2 = make([][]float32, nl)
	m.LayerFFNB2 = make([][]float32, nl)
	m.LayerLN2W = make([][]float32, nl)
	m.LayerLN2B = make([][]float32, nl)

	for l := 0; l < nl; l++ {
		if m.LayerAttnQKVW[l], err = loadFloat32s(f, hs*3*hs); err != nil {
			return nil, fmt.Errorf("layer %d attn_qkv_weight: %w", l, err)
		}
		if m.LayerAttnQKVB[l], err = loadFloat32s(f, 3*hs); err != nil {
			return nil, fmt.Errorf("layer %d attn_qkv_bias: %w", l, err)
		}
		if m.LayerAttnOutW[l], err = loadFloat32s(f, hs*hs); err != nil {
			return nil, fmt.Errorf("layer %d attn_out_weight: %w", l, err)
		}
		if m.LayerAttnOutB[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d attn_out_bias: %w", l, err)
		}
		if m.LayerLN1W[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d ln1_weight: %w", l, err)
		}
		if m.LayerLN1B[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d ln1_bias: %w", l, err)
		}
		if m.LayerFFNW1[l], err = loadFloat32s(f, hs*inter); err != nil {
			return nil, fmt.Errorf("layer %d ffn_w1: %w", l, err)
		}
		if m.LayerFFNB1[l], err = loadFloat32s(f, inter); err != nil {
			return nil, fmt.Errorf("layer %d ffn_b1: %w", l, err)
		}
		if m.LayerFFNW2[l], err = loadFloat32s(f, inter*hs); err != nil {
			return nil, fmt.Errorf("layer %d ffn_w2: %w", l, err)
		}
		if m.LayerFFNB2[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d ffn_b2: %w", l, err)
		}
		if m.LayerLN2W[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d ln2_weight: %w", l, err)
		}
		if m.LayerLN2B[l], err = loadFloat32s(f, hs); err != nil {
			return nil, fmt.Errorf("layer %d ln2_bias: %w", l, err)
		}
	}

	// Pooler.
	m.PoolerWeight, err = loadFloat32s(f, hs*hs)
	if err != nil {
		return nil, fmt.Errorf("read pooler_weight: %w", err)
	}
	m.PoolerBias, err = loadFloat32s(f, hs)
	if err != nil {
		return nil, fmt.Errorf("read pooler_bias: %w", err)
	}

	return m, nil
}