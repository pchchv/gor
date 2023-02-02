package gor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
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

// Use appends a middleware handler to the Mux middleware stack.
// The middleware stack for any Mux will execute before finding a matching route to a specific handler,
// which provides opportunity to respond early, change the course of the request execution,
// or set request-scoped values for the next http.Handler.
func (mx *Mux) Use(middlewares ...func(http.Handler) http.Handler) {
	if mx.handler != nil {
		panic("chi: all middlewares must be defined before routes on a mux")
	}

	mx.middlewares = append(mx.middlewares, middlewares...)
}

// Method adds a route `pattern` that matches `method` http method to execute the `handler` http.Handler.
func (mx *Mux) Method(method, pattern string, handler http.Handler) {
	m, ok := methodMap[strings.ToUpper(method)]
	if !ok {
		panic(fmt.Sprintf("chi: '%s' http method is not supported.", method))
	}

	mx.handle(m, pattern, handler)
}

// MethodFunc adds a route `pattern` that matches `method` http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) MethodFunc(method, pattern string, handlerFn http.HandlerFunc) {
	mx.Method(method, pattern, handlerFn)
}

// MethodNotAllowedHandler returns the default Mux 405 responder whenever a method cannot be resolved for a route.
func (mx *Mux) MethodNotAllowedHandler() http.HandlerFunc {
	if mx.methodNotAllowedHandler != nil {
		return mx.methodNotAllowedHandler
	}
	return methodNotAllowedHandler
}

// Handle adds a `pattern` route matching any http method to execute `handler` http.Handler.
func (mx *Mux) Handle(pattern string, handler http.Handler) {
	mx.handle(mALL, pattern, handler)
}

// HandleFunc adds a `pattern` route that matches any http method to. execute `handlerFn` http.HandlerFunc.
func (mx *Mux) HandleFunc(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mALL, pattern, handlerFn)
}

// handle registers http.Handler in the routing tree for a particular http method and routing pattern.
func (mx *Mux) handle(method methodType, pattern string, handler http.Handler) *node {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Sprintf("chi: routing pattern must begin with '/' in '%s'", pattern))
	}

	// build the computed routing handler for this routing pattern
	if !mx.inline && mx.handler == nil {
		mx.updateRouteHandler()
	}

	// build endpoint handler with inline middlewares for the route
	var h http.Handler
	if mx.inline {
		mx.handler = http.HandlerFunc(mx.routeHTTP)
		h = Chain(mx.middlewares...).Handler(handler)
	} else {
		h = handler
	}

	// add the endpoint to the tree and return the node
	return mx.tree.InsertRoute(method, pattern, h)
}

// routeHTTP routes a http.Request through the Mux routing tree to serve the matching handler for a particular http method.
func (mx *Mux) routeHTTP(w http.ResponseWriter, r *http.Request) {
	// grab the route context object
	rctx := r.Context().Value(RouteCtxKey).(*Context)

	// the request routing path
	routePath := rctx.RoutePath
	if routePath == "" {
		if r.URL.RawPath != "" {
			routePath = r.URL.RawPath
		} else {
			routePath = r.URL.Path
		}

		if routePath == "" {
			routePath = "/"
		}
	}

	// check if method is supported by gor
	if rctx.RouteMethod == "" {
		rctx.RouteMethod = r.Method
	}

	method, ok := methodMap[rctx.RouteMethod]
	if !ok {
		mx.MethodNotAllowedHandler().ServeHTTP(w, r)
		return
	}

	// find the route
	if _, _, h := mx.tree.FindRoute(rctx, method, routePath); h != nil {
		h.ServeHTTP(w, r)
		return
	}
	if rctx.methodNotAllowed {
		mx.MethodNotAllowedHandler().ServeHTTP(w, r)
	} else {
		mx.NotFoundHandler().ServeHTTP(w, r)
	}
}

// updateRouteHandler builds a single mux handler, which is a chain of middlewares stack defined by Use() calls,
// and the tree router (Mux) itself. After this point, no other middleware can be registered in the stack of this Mux.
// But it is still possible to link additional middlewares through Group() or using the chain of middleware handlers.
func (mx *Mux) updateRouteHandler() {
	mx.handler = chain(mx.middlewares, http.HandlerFunc(mx.routeHTTP))
}

// methodNotAllowedHandler is a helper function to respond with a 405, method not allowed.
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(405)
	w.Write(nil)
}
