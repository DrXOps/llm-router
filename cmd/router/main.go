package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/user/llm-router/internal/config"
	"github.com/user/llm-router/internal/proxy"
	"github.com/user/llm-router/internal/router"
	"github.com/user/llm-router/internal/slm"
)

var version = "0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "show version and exit")
	configPath := flag.String("config", "", "path to config.yaml (default: config.yaml in current directory)")
	listenAddr := flag.String("addr", ":8080", "listen address")
	flag.Parse()

	if *showVersion {
		fmt.Println("llm-router version", version)
		os.Exit(0)
	}

	// Load configuration.
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup structured logging.
	var slogLevel slog.Level
	switch cfg.Logging.Level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(slogLevel)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
	})))

	slog.Info("starting llm-router",
		"version", version,
		"listen", *listenAddr,
		"upstream", cfg.Upstream.BaseURL,
		"rules_dir", cfg.Routing.RulesDir,
	)

	// Initialize SLM classifier (optional, NLP or graceful degradation).
	classifier := slm.NewClassifier(
		cfg.Routing.EnableSLMClassifier,
		cfg.Routing.SLMModelPath,
		cfg.Routing.SLMVocabPath,
	)
	if classifier.Enabled() {
		slog.Info("SLM NLP classifier loaded", "info", classifier.DebugInfo())
	}

	// Build router from config and rules_dir.
	rt, err := router.LoadAndBuildRouter(cfg)
	if err != nil {
		slog.Error("failed to build router", "error", err)
		os.Exit(1)
	}
	slog.Info("router initialized", "rules_count", len(rt.Rules()))

	// Wrap in atomic container for safe hot-reload.
	atomicRouter := router.NewAtomicRouter(rt)

	// Create proxy handler.
	handler := proxy.Handler(cfg, atomicRouter, classifier)

	// Setup HTTP server.
	server := &http.Server{
		Addr:         *listenAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 5 * time.Minute,
	}

	// Handle graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Hot-reload rules on SIGHUP.
	if cfg.Routing.WatchRules {
		reloadCh := make(chan os.Signal, 1)
		signal.Notify(reloadCh, syscall.SIGHUP)
		go func() {
			for range reloadCh {
				slog.Info("SIGHUP received, reloading rules")
				newRt, err := router.LoadAndBuildRouter(cfg)
				if err != nil {
					slog.Error("failed to reload rules", "error", err)
					continue
				}
				atomicRouter.Store(newRt)
				slog.Info("rules atomically reloaded", "rules_count", len(newRt.Rules()))
			}
		}()
	}

	// Run server.
	go func() {
		slog.Info("listening", "addr", *listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}