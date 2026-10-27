package proxy

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/user/llm-router/internal/config"
	"github.com/user/llm-router/internal/payload"
	"github.com/user/llm-router/internal/router"
	"github.com/user/llm-router/internal/sanitizer"
)

// Classifier is the interface for optional NLP-based model suggestion.
type Classifier interface {
	Classify(lastUserMessage string) (target string, confidence float32)
	Enabled() bool
}

// Handler creates the main HTTP handler for the llm-router proxy.
// rt is an AtomicRouter, safe for concurrent reads and hot-reload swaps.
// classifier is optional; pass nil to skip NLP classification.
func Handler(cfg *config.Config, rt *router.AtomicRouter, classifier Classifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("failed to read request body", "error", err)
			http.Error(w, "failed to read request body", http.StatusInternalServerError)
			return
		}
		r.Body.Close()

		if cfg.Logging.LogRequestBody {
			slog.Debug("request body", "body", string(body))
		}

		p, err := payload.Parse(body)
		if err != nil {
			slog.Error("failed to parse request body", "error", err)
			http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
			return
		}

		modelIDs := make(map[string]string)
		for key, m := range cfg.Models {
			modelIDs[key] = m.ID
		}

		ctx := router.NewPayloadCtx(modelIDs)
		ctx.HasToolCalls = p.HasToolCalls()
		ctx.LastUserMessage = p.LastUserMessage()
		ctx.AllUserMessages = p.AllUserMessages()
		ctx.TotalChars = p.TotalChars()
		ctx.ApproxTokens = payload.ApproxTokensFromChars(p.TotalChars())
		ctx.MessageCount = p.CountMessages()

		// Step 1: NLP classifier (optional).
		var result router.Result
		nlpMatched := false
		if classifier != nil && classifier.Enabled() {
			nlpTarget, nlpConfidence := classifier.Classify(ctx.LastUserMessage)
			if nlpTarget != "" && nlpConfidence >= 0.7 {
				result = router.MatchResult(nlpTarget, fmt.Sprintf("NLP classifier: %.2f confidence", nlpConfidence))
				nlpMatched = true
				if cfg.Logging.LogModelChoice {
					slog.Info("NLP model selected",
						"target", nlpTarget,
						"confidence", nlpConfidence,
					)
				}
			}
		}

		// Step 2: Rules (override NLP only if they match).
		if !nlpMatched {
			result = rt.Load().Eval(ctx)
			if !result.Match {
				slog.Warn("no routing rule matched, using fallback",
					"message_count", ctx.MessageCount)
				result = router.MatchResult("haiku", "default fallback")
			}
		}

		newModelID := result.Target
		if id, ok := cfg.Models[result.Target]; ok {
			newModelID = id.ID
		}

		if cfg.Logging.LogModelChoice {
			slog.Info("model selected",
				"target", result.Target,
				"model_id", newModelID,
				"reason", result.Reason)
		}

		// Apply sanitization for DeepSeek.
		sanitizedBody := body
		if result.Target == "deepseek" {
			sanitizedBody, err = sanitizer.SanitizeForModel(body, "deepseek")
			if err != nil {
				slog.Warn("sanitization failed, using original body", "error", err)
				sanitizedBody = body
			}
		}

		if string(sanitizedBody) != string(body) {
			p, err = payload.Parse(sanitizedBody)
			if err != nil {
				slog.Error("failed to re-parse sanitized body", "error", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}

		newBody, err := payload.Rebuild(p, newModelID)
		if err != nil {
			slog.Error("failed to rebuild payload", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		upstreamURL, err := url.Parse(cfg.Upstream.BaseURL)
		if err != nil {
			slog.Error("invalid upstream URL", "error", err)
			http.Error(w, "invalid upstream URL", http.StatusInternalServerError)
			return
		}

		revProxy := httputil.NewSingleHostReverseProxy(upstreamURL)
		revProxy.Director = func(req *http.Request) {
			req.URL.Scheme = upstreamURL.Scheme
			req.URL.Host = upstreamURL.Host
			req.URL.Path = "/v1/messages"
			req.Host = upstreamURL.Host

			authHeader := r.Header.Get("X-Api-Key")
			if authHeader == "" {
				authHeader = r.Header.Get("x-api-key")
			}
			if authHeader != "" {
				req.Header.Set("x-api-key", authHeader)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Body = io.NopCloser(bytes.NewReader(newBody))
			req.ContentLength = int64(len(newBody))
		}

		revProxy.ServeHTTP(w, r)
	})
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}