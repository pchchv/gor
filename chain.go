package gor

import "net/http"

// ChainHandler is a http.Handler with support for handler composition and execution.
type ChainHandler struct {
	Endpoint    http.Handler
	chain       http.Handler
	Middlewares Middlewares
}

// Chain returns a Middlewares type from a slice of middleware handlers.
func Chain(middlewares ...func(http.Handler) http.Handler) Middlewares {
	return Middlewares(middlewares)
}

func (c *ChainHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.chain.ServeHTTP(w, r)
}

// chain builds a http.Handler composed of an embedded middleware stack and
// an endpoint handler in the order in which they are passed.
func chain(middlewares []func(http.Handler) http.Handler, endpoint http.Handler) http.Handler {
	// return early if there are no any middlewares for the chain
	if len(middlewares) == 0 {
		return endpoint
	}

	// wrap the end handler with the middleware chain
	h := middlewares[len(middlewares)-1](endpoint)
	for i := len(middlewares) - 2; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}
