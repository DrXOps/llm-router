package router

import "sync/atomic"

// AtomicRouter wraps *Router for safe concurrent reads and hot-reload swaps.
type AtomicRouter struct {
	v atomic.Value
}

// NewAtomicRouter wraps an initial *Router.
func NewAtomicRouter(rt *Router) *AtomicRouter {
	ar := &AtomicRouter{}
	ar.v.Store(rt)
	return ar
}

// Load returns the current router for reading.
func (ar *AtomicRouter) Load() *Router {
	return ar.v.Load().(*Router)
}

// Store atomically replaces the router.
func (ar *AtomicRouter) Store(rt *Router) {
	ar.v.Store(rt)
}