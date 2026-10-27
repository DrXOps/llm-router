#!/usr/bin/env python3
"""
Export Multilingual-MiniLM-L6-v2 to custom .bin format for pure-Go inference.

Usage:
    pip install sentence-transformers torch numpy
    python scripts/export_model.py --output model.bin --vocab vocab.txt

Downloads the model from HuggingFace, extracts weights, and writes in
the binary format that internal/slm/weights.go reads.
"""

import argparse
import struct
import numpy as np
from sentence_transformers import SentenceTransformer

def export(model_name="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
           output="model.bin", vocab_output="vocab.txt"):
    print(f"Loading model: {model_name}")
    model = SentenceTransformer(model_name)

    # Access the underlying transformer.
    transformer = model._first_module().auto_model
    config = transformer.config

    vocab_size = config.vocab_size
    hidden_size = config.hidden_size
    num_layers = config.num_hidden_layers
    num_heads = config.num_attention_heads
    intermediate = config.intermediate_size
    max_seq_len = 256  # we use 256

    print(f"Vocab: {vocab_size}, Hidden: {hidden_size}, Layers: {num_layers}, Heads: {num_heads}, FFN: {intermediate}")

    with open(output, "wb") as f:
        # Header
        f.write(struct.pack("<i", vocab_size))
        f.write(struct.pack("<i", hidden_size))
        f.write(struct.pack("<i", num_layers))
        f.write(struct.pack("<i", num_heads))
        f.write(struct.pack("<i", intermediate))
        f.write(struct.pack("<i", max_seq_len))

        def write(name, tensor):
            """Write tensor as float32 to file."""
            t = tensor.detach().cpu().numpy().astype(np.float32).flatten()
            print(f"  {name}: {t.shape[0]} floats")
            f.write(t.tobytes())

        # Token embeddings (word_embeddings.weight)
        write("token_embeddings", transformer.embeddings.word_embeddings.weight)

        # Positional embeddings
        pos_emb = transformer.embeddings.position_embeddings.weight[:max_seq_len, :]
        write("pos_embeddings", pos_emb)

        # Embedding LayerNorm
        write("embed_ln_weight", transformer.embeddings.LayerNorm.weight)
        write("embed_ln_bias", transformer.embeddings.LayerNorm.bias)

        # Per-layer weights
        for i, layer in enumerate(transformer.encoder.layer):
            prefix = f"layer_{i}"
            attn = layer.attention.self
            out = layer.attention.output
            inter = layer.intermediate
            out_dense = layer.output

            # QKV weight: hidden × 3*hidden (fused)
            q = attn.query.weight
            k = attn.key.weight
            v = attn.value.weight
            qkv_w = torch.cat([q, k, v], dim=0).T  # hidden × 3*hidden
            qkv_b = torch.cat([attn.query.bias, attn.key.bias, attn.value.bias], dim=0)

            write(f"{prefix}.attn_qkv_w", qkv_w)
            write(f"{prefix}.attn_qkv_b", qkv_b)

            # Attention output
            write(f"{prefix}.attn_out_w", out.dense.weight.T)
            write(f"{prefix}.attn_out_b", out.dense.bias)
            write(f"{prefix}.attn_ln_w", out.LayerNorm.weight)
            write(f"{prefix}.attn_ln_b", out.LayerNorm.bias)

            # FFN
            write(f"{prefix}.ffn_w1", inter.dense.weight.T)
            write(f"{prefix}.ffn_b1", inter.dense.bias)
            write(f"{prefix}.ffn_w2", out_dense.dense.weight.T)
            write(f"{prefix}.ffn_b2", out_dense.dense.bias)

            # Post-FFN LayerNorm
            write(f"{prefix}.ffn_ln_w", out_dense.LayerNorm.weight)
            write(f"{prefix}.ffn_ln_b", out_dense.LayerNorm.bias)

        # Pooler
        write("pooler_weight", transformer.pooler.dense.weight.T)
        write("pooler_bias", transformer.pooler.dense.bias)

    print(f"\nModel saved: {output} ({os.path.getsize(output) / 1024 / 1024:.1f} MB)")

    # Export vocab (tokenizer)
    tokenizer = model._first_module().tokenizer
    if hasattr(tokenizer, "vocab"):
        vocab = tokenizer.vocab
    else:
        vocab = tokenizer.get_vocab()

    with open(vocab_output, "w", encoding="utf-8") as f:
        # Sort by id and write each token
        for token, idx in sorted(vocab.items(), key=lambda x: x[1]):
            f.write(token + "\n")

    print(f"Vocab saved: {vocab_output} ({len(vocab)} tokens)")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2")
    parser.add_argument("--output", default="model.bin")
    parser.add_argument("--vocab", default="vocab.txt")
    args = parser.parse_args()

    import os, torch  # noqa
    export(args.model, args.output, args.vocab)