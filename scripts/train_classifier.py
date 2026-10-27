#!/usr/bin/env python3
"""
Train a routing classifier on top of Multilingual-MiniLM-L12-v2.

Produces:
  model.bin    — MiniLM weights in custom binary format (for Go inference)
  vocab.txt    — WordPiece vocabulary (for tokenizer)
  head.bin     — Trained logistic regression weights (for classification head)

Usage:
    pip install torch sentence-transformers numpy scikit-learn tqdm
    python scripts/train_classifier.py
"""

import argparse
import json
import os
import struct
import sys
import numpy as np
import torch
from sentence_transformers import SentenceTransformer
from sklearn.linear_model import LogisticRegression
from sklearn.model_selection import train_test_split
from sklearn.metrics import classification_report, accuracy_score
from tqdm import tqdm

# ─── Training dataset ─────────────────────────────────────────────────

# Bilingual (Russian + English) examples for each target model.
TRAINING_DATA = [
    # ── Haiku: short, simple, creative, greetings ──
    {"text": "Привет, как дела?", "target": "haiku"},
    {"text": "Hello, how are you?", "target": "haiku"},
    {"text": "Напиши стих про весну", "target": "haiku"},
    {"text": "Write a poem about spring", "target": "haiku"},
    {"text": "Расскажи анекдот", "target": "haiku"},
    {"text": "Tell me a joke", "target": "haiku"},
    {"text": "Что такое любовь?", "target": "haiku"},
    {"text": "What is love?", "target": "haiku"},
    {"text": "Какая сегодня погода?", "target": "haiku"},
    {"text": "What's the weather like today?", "target": "haiku"},
    {"text": "Переведи слово 'hello' на русский", "target": "haiku"},
    {"text": "Translate 'привет' to English", "target": "haiku"},
    {"text": "Спасибо!", "target": "haiku"},
    {"text": "Thank you!", "target": "haiku"},
    {"text": "Придумай имя для кота", "target": "haiku"},
    {"text": "Suggest a name for my cat", "target": "haiku"},
    {"text": "Напиши короткий рассказ о дружбе", "target": "haiku"},
    {"text": "Write a short story about friendship", "target": "haiku"},
    {"text": "Объясни простыми словами что такое гравитация", "target": "haiku"},
    {"text": "Explain gravity in simple terms", "target": "haiku"},
    {"text": "Сколько будет 2+2?", "target": "haiku"},
    {"text": "What is 2+2?", "target": "haiku"},
    {"text": "Напиши рецепт блинов", "target": "haiku"},
    {"text": "Write a pancake recipe", "target": "haiku"},
    {"text": "Какие фильмы посмотреть?", "target": "haiku"},
    {"text": "What movies do you recommend?", "target": "haiku"},
    {"text": "Сделай комплимент", "target": "haiku"},
    {"text": "Give me a compliment", "target": "haiku"},
    {"text": "Расскажи интересный факт", "target": "haiku"},
    {"text": "Tell me an interesting fact", "target": "haiku"},
    {"text": "Напиши письмо другу", "target": "haiku"},
    {"text": "Write a letter to a friend", "target": "haiku"},
    {"text": "Как пройти в библиотеку?", "target": "haiku"},
    {"text": "Directions to the library?", "target": "haiku"},
    {"text": "Краткое содержание 'Войны и мира'", "target": "haiku"},
    {"text": "Summary of 'War and Peace'", "target": "haiku"},
    {"text": "Приветствие для конференции", "target": "haiku"},
    {"text": "Opening speech for a conference", "target": "haiku"},
    {"text": "Мотивационная цитата", "target": "haiku"},
    {"text": "Inspirational quote", "target": "haiku"},
    {"text": "Тост на свадьбу", "target": "haiku"},
    {"text": "Wedding toast", "target": "haiku"},

    # ── DeepSeek: complex engineering, code, architecture ──
    {"text": "Спроектируй архитектуру микросервисной системы на Go", "target": "deepseek"},
    {"text": "Design a microservice architecture in Go", "target": "deepseek"},
    {"text": "Реализуй rate limiter на Go с паттерном Token Bucket", "target": "deepseek"},
    {"text": "Implement a token bucket rate limiter in Go", "target": "deepseek"},
    {"text": "Напиши production-grade Kubernetes оператор на Go", "target": "deepseek"},
    {"text": "Write a production-grade Kubernetes operator in Go", "target": "deepseek"},
    {"text": "Рефакторинг: раздели монолит на микросервисы", "target": "deepseek"},
    {"text": "Refactoring: split monolith into microservices", "target": "deepseek"},
    {"text": "Оптимизируй SQL запрос с JOIN и подзапросами", "target": "deepseek"},
    {"text": "Optimize SQL query with JOINs and subqueries", "target": "deepseek"},
    {"text": "Спроектируй API для платежной системы с REST и gRPC", "target": "deepseek"},
    {"text": "Design payment system API with REST and gRPC", "target": "deepseek"},
    {"text": "Почини баг в парсере конфигов на Python", "target": "deepseek"},
    {"text": "Fix a bug in Python config parser", "target": "deepseek"},
    {"text": "Напиши Terraform модуль для AWS EKS", "target": "deepseek"},
    {"text": "Write Terraform module for AWS EKS", "target": "deepseek"},
    {"text": "Реализуй алгоритм консенсуса Raft на Go", "target": "deepseek"},
    {"text": "Implement Raft consensus algorithm in Go", "target": "deepseek"},
    {"text": "Деплоймент стратегии для Kubernetes: blue-green vs canary", "target": "deepseek"},
    {"text": "Kubernetes deployment strategies: blue-green vs canary", "target": "deepseek"},
    {"text": "Анализ производительности: профилируй CPU и память", "target": "deepseek"},
    {"text": "Performance analysis: profile CPU and memory", "target": "deepseek"},
    {"text": "Реализуй CI/CD pipeline для монорепозитория", "target": "deepseek"},
    {"text": "Implement CI/CD pipeline for monorepo", "target": "deepseek"},
    {"text": "Напиши код для работы с Apache Kafka на Java", "target": "deepseek"},
    {"text": "Write Apache Kafka client code in Java", "target": "deepseek"},
    {"text": "Проектируй систему логирования на ELK стеке", "target": "deepseek"},
    {"text": "Design logging system with ELK stack", "target": "deepseek"},
    {"text": "Реализуй GraphQL resolver с PostgreSQL и DataLoader", "target": "deepseek"},
    {"text": "Implement GraphQL resolver with PostgreSQL and DataLoader", "target": "deepseek"},
    {"text": "Настройка nginx как reverse proxy для микросервисов", "target": "deepseek"},
    {"text": "Configure nginx as reverse proxy for microservices", "target": "deepseek"},
    {"text": "Оптимизируй Docker образ: multi-stage сборка", "target": "deepseek"},
    {"text": "Optimize Docker image with multi-stage build", "target": "deepseek"},
    {"text": "Реализуй свой HTTP роутер на Go с нуля", "target": "deepseek"},
    {"text": "Implement your own HTTP router in Go from scratch", "target": "deepseek"},
    {"text": "Архитектура event-driven систем на RabbitMQ", "target": "deepseek"},
    {"text": "Event-driven architecture with RabbitMQ", "target": "deepseek"},
    {"text": "Напиши prometheus exporter для кастомных метрик", "target": "deepseek"},
    {"text": "Write a Prometheus exporter for custom metrics", "target": "deepseek"},
    {"text": "Автоматизация деплоя с Ansible и GitHub Actions", "target": "deepseek"},
    {"text": "Automate deployment with Ansible and GitHub Actions", "target": "deepseek"},
]

# ─── Export functions ─────────────────────────────────────────────────

def export_model_bin(model, output_path, max_seq_len=256):
    """Export MiniLM weights to custom binary format."""
    transformer = model._first_module().auto_model
    config = transformer.config

    vocab_size = config.vocab_size
    hidden_size = config.hidden_size
    num_layers = config.num_hidden_layers
    num_heads = config.num_attention_heads
    intermediate = config.intermediate_size

    print(f"\nExporting model: vocab={vocab_size} hidden={hidden_size} layers={num_layers} heads={num_heads} ffn={intermediate}")

    with open(output_path, "wb") as f:
        def write(name, tensor):
            t = tensor.detach().cpu().numpy().astype(np.float32).flatten()
            f.write(t.tobytes())
            print(f"  {name}: {t.shape[0]} floats")

        # Header
        for val in [vocab_size, hidden_size, num_layers, num_heads, intermediate, max_seq_len]:
            f.write(struct.pack("<i", val))

        # Embeddings
        write("token_embeddings", transformer.embeddings.word_embeddings.weight)
        pos_emb = transformer.embeddings.position_embeddings.weight[:max_seq_len, :]
        write("pos_embeddings", pos_emb)
        write("embed_ln_weight", transformer.embeddings.LayerNorm.weight)
        write("embed_ln_bias", transformer.embeddings.LayerNorm.bias)

        # Layers
        for i, layer in enumerate(transformer.encoder.layer):
            prefix = f"layer_{i}"
            attn = layer.attention.self
            attn_out = layer.attention.output
            inter = layer.intermediate
            out = layer.output

            qkv_w = torch.cat([attn.query.weight, attn.key.weight, attn.value.weight], dim=0).T
            qkv_b = torch.cat([attn.query.bias, attn.key.bias, attn.value.bias], dim=0)
            write(f"{prefix}.attn_qkv_w", qkv_w)
            write(f"{prefix}.attn_qkv_b", qkv_b)
            write(f"{prefix}.attn_out_w", attn_out.dense.weight.T)
            write(f"{prefix}.attn_out_b", attn_out.dense.bias)
            write(f"{prefix}.attn_ln_w", attn_out.LayerNorm.weight)
            write(f"{prefix}.attn_ln_b", attn_out.LayerNorm.bias)
            write(f"{prefix}.ffn_w1", inter.dense.weight.T)
            write(f"{prefix}.ffn_b1", inter.dense.bias)
            write(f"{prefix}.ffn_w2", out.dense.weight.T)
            write(f"{prefix}.ffn_b2", out.dense.bias)
            write(f"{prefix}.ffn_ln_w", out.LayerNorm.weight)
            write(f"{prefix}.ffn_ln_b", out.LayerNorm.bias)

        # Pooler
        write("pooler_weight", transformer.pooler.dense.weight.T)
        write("pooler_bias", transformer.pooler.dense.bias)

    mb = os.path.getsize(output_path) / 1024 / 1024
    print(f"\n✅ Model exported: {output_path} ({mb:.1f} MB)")


def export_vocab(model, output_path):
    """Export tokenizer vocabulary as text file (one token per line)."""
    tokenizer = model._first_module().tokenizer
    vocab = tokenizer.get_vocab()
    # Sort by id
    sorted_tokens = sorted(vocab.items(), key=lambda x: x[1])
    with open(output_path, "w", encoding="utf-8") as f:
        for token, _ in sorted_tokens:
            f.write(token + "\n")
    print(f"✅ Vocab exported: {output_path} ({len(sorted_tokens)} tokens)")


def export_head(head, label_map, path):
    """Export trained logistic regression head weights to binary format."""
    weights = head.coef_.astype(np.float32)  # [num_classes × hidden_size]
    bias = head.intercept_.astype(np.float32)  # [num_classes]
    labels = list(label_map.keys())

    with open(path, "wb") as f:
        # Header: num_classes, hidden_size
        f.write(struct.pack("<ii", len(labels), weights.shape[1]))
        # Labels
        for lbl in labels:
            lbl_bytes = lbl.encode("utf-8") + b"\x00"
            f.write(struct.pack("<i", len(lbl_bytes)))
            f.write(lbl_bytes)
        # Weights
        f.write(weights.tobytes())
        f.write(bias.tobytes())

    print(f"✅ Classification head exported: {path}")
    print(f"   Classes: {labels}")
    print(f"   Weights shape: {weights.shape}")


# ─── Main ──────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Train LLM router classifier")
    parser.add_argument("--model", default="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
                        help="HuggingFace model name")
    parser.add_argument("--output-dir", default=".",
                        help="Output directory for model files")
    parser.add_argument("--device", default="cuda" if torch.cuda.is_available() else "cpu",
                        help="Device: cuda or cpu")
    parser.add_argument("--epochs", type=int, default=10,
                        help="Number of training epochs (only used if fine-tuning)")
    parser.add_argument("--batch-size", type=int, default=16)
    args = parser.parse_args()

    device = args.device
    print(f"Using device: {device}")
    print(f"CUDA available: {torch.cuda.is_available()}")
    if torch.cuda.is_available():
        print(f"GPU: {torch.cuda.get_device_name(0)}")

    # Load model
    print(f"\nLoading model: {args.model}")
    model = SentenceTransformer(args.model, device=device)
    hidden_size = model._first_module().auto_model.config.hidden_size

    # Extract embeddings for training data
    texts = [item["text"] for item in TRAINING_DATA]
    labels = [item["target"] for item in TRAINING_DATA]
    label_map = {"haiku": 0, "deepseek": 1}
    y = np.array([label_map[l] for l in labels])

    print(f"\nEncoding {len(texts)} training examples...")
    embeddings = model.encode(texts, batch_size=args.batch_size, show_progress_bar=True,
                              convert_to_numpy=True, normalize_embeddings=True)

    # Train/test split
    X_train, X_test, y_train, y_test = train_test_split(
        embeddings, y, test_size=0.2, random_state=42, stratify=y
    )

    # Train logistic regression
    print("\nTraining logistic regression classifier...")
    clf = LogisticRegression(
        C=1.0,
        max_iter=1000,
        class_weight="balanced",
        multi_class="multinomial",
        solver="lbfgs",
        random_state=42,
    )
    clf.fit(X_train, y_train)

    # Evaluate
    y_pred = clf.predict(X_test)
    acc = accuracy_score(y_test, y_pred)
    print(f"\n📊 Test accuracy: {acc:.3f}")
    print("\nClassification report:")
    print(classification_report(y_test, y_pred, target_names=["haiku", "deepseek"]))

    # Export files
    os.makedirs(args.output_dir, exist_ok=True)
    export_model_bin(model, os.path.join(args.output_dir, "model.bin"))
    export_vocab(model, os.path.join(args.output_dir, "vocab.txt"))
    export_head(clf, {"haiku": 0, "deepseek": 1}, os.path.join(args.output_dir, "head.bin"))

    print(f"\n🎉 All files exported to {args.output_dir}/")

    # Print usage summary
    print("""
Files to place next to llm-router binary:
  model.bin  (~22 MB) — MiniLM weights
  vocab.txt  (~1 MB)  — tokenizer vocabulary
  head.bin   (~3 KB)  — trained classification head

Set in config.yaml:
  routing:
    enable_slm_classifier: true
    slm_model_path: "model.bin"
    slm_vocab_path: "vocab.txt"
    slm_head_path: "head.bin"
""")


if __name__ == "__main__":
    main()