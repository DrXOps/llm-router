package router

import (
	"sync"
	"testing"
)

func TestAtomicRouterLoadStore(t *testing.T) {
	rt1 := NewRouter()
	rt1.AddRule(NewAlwaysRule("a", 1, "haiku"))

	ar := NewAtomicRouter(rt1)

	// Load should return the same router.
	loaded := ar.Load()
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if len(loaded.Rules()) != 1 {
		t.Errorf("expected 1 rule, got %d", len(loaded.Rules()))
	}

	// Store a new router.
	rt2 := NewRouter()
	rt2.AddRule(NewAlwaysRule("b", 2, "deepseek"))
	ar.Store(rt2)

	loaded2 := ar.Load()
	if len(loaded2.Rules()) != 1 {
		t.Errorf("expected 1 rule, got %d", len(loaded2.Rules()))
	}
	name := loaded2.Rules()[0].Name()
	if name != "b" {
		t.Errorf("expected rule 'b', got %q", name)
	}
}

func TestAtomicRouterConcurrent(t *testing.T) {
	rt := NewRouter()
	rt.AddRule(NewAlwaysRule("base", 0, "haiku"))
	ar := NewAtomicRouter(rt)

	var wg sync.WaitGroup

	// Multiple concurrent readers.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				r := ar.Load()
				_ = r.Rules()
			}
		}()
	}

	// Concurrent writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			newRt := NewRouter()
			newRt.AddRule(NewAlwaysRule("x", j, "haiku"))
			ar.Store(newRt)
		}
	}()

	wg.Wait()
	// No panics = test passes for atomic safety.
}