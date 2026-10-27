package router

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/DrXOps/llm-router/internal/config"
)

// WatchRules polls the rules directory every interval and returns a channel
// that receives reload notifications when changes are detected.
// On Windows, this is the primary hot-reload mechanism (no SIGHUP).
func WatchRules(cfg *config.Config, interval time.Duration) chan struct{} {
	reload := make(chan struct{}, 1)

	if !cfg.Routing.WatchRules || cfg.Routing.RulesDir == "" {
		slog.Debug("rule watching disabled")
		close(reload)
		return reload
	}

	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	go func() {
		// Track file modification times.
		modTimes := make(map[string]time.Time)

		// Initial scan.
		_ = scanDirModTimes(cfg.Routing.RulesDir, modTimes)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			current := make(map[string]time.Time)
			if err := scanDirModTimes(cfg.Routing.RulesDir, current); err != nil {
				continue
			}

			changed := false
			for path, mtime := range current {
				old, exists := modTimes[path]
				if !exists || !old.Equal(mtime) {
					changed = true
					break
				}
			}
			// Also check for deleted files.
			if !changed {
				for path := range modTimes {
					if _, exists := current[path]; !exists {
						changed = true
						break
					}
				}
			}

			if changed {
				slog.Info("rules directory changed, triggering reload")
				select {
				case reload <- struct{}{}:
				default:
					// Channel already has a pending notification.
				}
				modTimes = current
			}
		}
	}()

	return reload
}

// scanDirModTimes reads all rule files and records their modification times.
func scanDirModTimes(dir string, out map[string]time.Time) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".star" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out[filepath.Join(dir, entry.Name())] = info.ModTime()
	}

	return nil
}
