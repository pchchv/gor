package gor

import (
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
