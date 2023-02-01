package gor

import (
	"context"
	"net/http"
	"sync"
)

// Mux is a simple HTTP route multiplexer that parses the request path,
// writes any URL parameters and executes the end handler.
// It implements the http.Handler interface and is friendly with the standard library.
type Mux struct {
	// Computed mux handler  consisting of a middleware stack and the tree router
	handler http.Handler

	// The radix trie router
	tree *node

	// Custom method not allowed handler
	methodNotAllowedHandler http.HandlerFunc

	// Reference to the parent mux used by subrouters when mounting to a parent mux
	parent *Mux

	// Routing context pool
	pool *sync.Pool

	// Custom route not found handler
	notFoundHandler http.HandlerFunc

	// The middleware stack
	middlewares []func(http.Handler) http.Handler

	// Controls the middleware chain generation behavior when an mux registers
	// as an inline group within another mux.
	inline bool
}

// NewMux returns a newly initialized Mux object that implements the Router interface.
func NewMux() *Mux {
	mux := &Mux{tree: &node{}, pool: &sync.Pool{}}

	mux.pool.New = func() interface{} {
		return NewRouteContext()
	}

	return mux
}

// NotFoundHandler returns the default Mux 404 responder whenever a route cannot be found.
func (mx *Mux) NotFoundHandler() http.HandlerFunc {
	if mx.notFoundHandler != nil {
		return mx.notFoundHandler
	}

	return http.NotFound
}

// ServeHTTP is the only method of the http.Handler interface that
// makes Mux compatible with the standard library.
// It uses sync.Pool to get and reuse routing contexts for each request.
func (mx *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Ensure the mux has some routes defined on the mux
	if mx.handler == nil {
		mx.NotFoundHandler().ServeHTTP(w, r)
		return
	}

	// Check if the routing context from the parent router already exists.
	rctx, _ := r.Context().Value(RouteCtxKey).(*Context)
	if rctx != nil {
		mx.handler.ServeHTTP(w, r)
		return
	}

	// Fetch a RouteContext object from the sync pool and call the computable
	// mx.handler consisting of mx.middlewares + mx.routeHTTP. When the request is finished,
	// reset the routing context and put it back in the pool for reuse from another request.
	rctx = mx.pool.Get().(*Context)
	rctx.Reset()
	rctx.Routes = mx
	rctx.parentCtx = r.Context()

	// r.WithContext() causes 2 allocations and context.WithValue() causes 1 allocation
	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))

	// Serve the request and after its done
	// return the request context back to the synchronization pool
	mx.handler.ServeHTTP(w, r)
	mx.pool.Put(rctx)
}
