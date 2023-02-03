package gor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/pchchv/golog"
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

var _ Router = &Mux{}

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
		panic("gor: all middlewares must be defined before routes on a mux")
	}

	mx.middlewares = append(mx.middlewares, middlewares...)
}

// Method adds a route `pattern` that matches `method` http method to execute the `handler` http.Handler.
func (mx *Mux) Method(method, pattern string, handler http.Handler) {
	m, ok := methodMap[strings.ToUpper(method)]
	if !ok {
		panic(fmt.Sprintf("gor: '%s' http method is not supported.", method))
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

// Get adds a route `pattern` that matches a GET http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Get(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mGET, pattern, handlerFn)
}

// Delete adds a route `pattern` that matches a DELETE http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Delete(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mDELETE, pattern, handlerFn)
}

// Connect adds a route `pattern` that matches a CONNECT http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Connect(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mCONNECT, pattern, handlerFn)
}

// Head adds a route `pattern` that matches a HEAD http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Head(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mHEAD, pattern, handlerFn)
}

// Options adds a route `pattern` that matches a OPTIONS http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Options(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mOPTIONS, pattern, handlerFn)
}

// Patch adds a route `pattern` that matches a PATCH http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Patch(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mPATCH, pattern, handlerFn)
}

// Post adds a route `pattern` that matches a POST http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Post(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mPOST, pattern, handlerFn)
}

// Put adds a route `pattern` that matches a PUT http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Put(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mPUT, pattern, handlerFn)
}

// Trace adds a route `pattern` that matches a TRACE http method to execute the `handlerFn` http.HandlerFunc.
func (mx *Mux) Trace(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mTRACE, pattern, handlerFn)
}

// NotFound sets a custom http.HandlerFunc to route paths that cannot be found.
// The default 404 handler is `http.NotFound`.
func (mx *Mux) NotFound(handlerFn http.HandlerFunc) {
	// build NotFound handler chain
	m := mx
	hFn := handlerFn
	if mx.inline && mx.parent != nil {
		m = mx.parent
		hFn = Chain(mx.middlewares...).HandlerFunc(hFn).ServeHTTP
	}

	// update a notFoundHandler from this point forward
	m.notFoundHandler = hFn
	m.updateSubRoutes(func(subMux *Mux) {
		if subMux.notFoundHandler == nil {
			subMux.NotFound(hFn)
		}
	})
}

// MethodNotAllowed sets a custom http.HandlerFunc to route paths where the method is unresolved.
// The default handler returns a 405 with an empty body.
func (mx *Mux) MethodNotAllowed(handlerFn http.HandlerFunc) {
	// build MethodNotAllowed handler chain
	m := mx
	hFn := handlerFn
	if mx.inline && mx.parent != nil {
		m = mx.parent
		hFn = Chain(mx.middlewares...).HandlerFunc(hFn).ServeHTTP
	}

	// update the methodNotAllowedHandler from this point forward
	m.methodNotAllowedHandler = hFn
	m.updateSubRoutes(func(subMux *Mux) {
		if subMux.methodNotAllowedHandler == nil {
			subMux.MethodNotAllowed(hFn)
		}
	})
}

// With adds inline middlewares for the endpoint handler.
func (mx *Mux) With(middlewares ...func(http.Handler) http.Handler) Router {
	// imilarly, as in handle(), a mux handler must be created when additional middleware registration
	// is not allowed for this stack, as it is now.
	if !mx.inline && mx.handler == nil {
		mx.updateRouteHandler()
	}

	// copy middlewares from parent inline muxs
	var mws Middlewares
	if mx.inline {
		mws = make(Middlewares, len(mx.middlewares))
		copy(mws, mx.middlewares)
	}
	mws = append(mws, middlewares...)

	im := &Mux{
		pool: mx.pool, inline: true, parent: mx, tree: mx.tree, middlewares: mws,
		notFoundHandler: mx.notFoundHandler, methodNotAllowedHandler: mx.methodNotAllowedHandler,
	}

	return im
}

// Handle adds a `pattern` route matching any http method to execute `handler` http.Handler.
func (mx *Mux) Handle(pattern string, handler http.Handler) {
	mx.handle(mALL, pattern, handler)
}

// HandleFunc adds a `pattern` route that matches any http method to. execute `handlerFn` http.HandlerFunc.
func (mx *Mux) HandleFunc(pattern string, handlerFn http.HandlerFunc) {
	mx.handle(mALL, pattern, handlerFn)
}

// Group creates a new inline-Mux with a fresh middleware stack.
// This is useful for a group of handlers on the same routing path that use an additional set of middleware.
func (mx *Mux) Group(fn func(r Router)) Router {
	im := mx.With().(*Mux)

	if fn != nil {
		fn(im)
	}

	return im
}

// Route creates a new Mux with a fresh middleware stack and mounts it along the `pattern` as a subrouter.
// This is essentially a shortened Mount call.
func (mx *Mux) Route(pattern string, fn func(r Router)) Router {
	if fn == nil {
		panic(fmt.Sprintf("gor: attempting to Route() a nil subrouter on '%s'", pattern))
	}

	subRouter := NewRouter()
	fn(subRouter)
	mx.Mount(pattern, subRouter)

	return subRouter
}

// Mount attaches another http.Handler or gor Router as a subrouter on the routing path.
// It is very useful to split a large API into many independent routers and merge them into a single service using Mount.
// Note that Mount() simply sets a wildcard in the `template` that will continue routing in the `handler`,
// which in most cases is another gor.Router.
// As a result, if you define two Mount() routes on the same pattern, mount will cause a panic.
func (mx *Mux) Mount(pattern string, handler http.Handler) {
	if handler == nil {
		panic(fmt.Sprintf("gor: attempting to Mount() a nil handler on '%s'", pattern))
	}

	// provide runtime safety for ensuring a pattern isn't mounted on an existing routing pattern.
	if mx.tree.findPattern(pattern+"*") || mx.tree.findPattern(pattern+"/*") {
		panic(fmt.Sprintf("gor: attempting to Mount() a handler on an existing path, '%s'", pattern))
	}

	// assign sub-Router's with the parent not found & method not allowed handler if not specified.
	subr, ok := handler.(*Mux)
	if ok && subr.notFoundHandler == nil && mx.notFoundHandler != nil {
		subr.NotFound(mx.notFoundHandler)
	}
	if ok && subr.methodNotAllowedHandler == nil && mx.methodNotAllowedHandler != nil {
		subr.MethodNotAllowed(mx.methodNotAllowedHandler)
	}

	mountHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := RouteContext(r.Context())

		// shift the url path past the previous subrouter
		rctx.RoutePath = mx.nextRoutePath(rctx)

		// reset the wildcard URLParam which connects the subrouter
		n := len(rctx.URLParams.Keys) - 1
		if n >= 0 && rctx.URLParams.Keys[n] == "*" && len(rctx.URLParams.Values) > n {
			rctx.URLParams.Values[n] = ""
		}

		handler.ServeHTTP(w, r)
	})

	if pattern == "" || pattern[len(pattern)-1] != '/' {
		mx.handle(mALL|mSTUB, pattern, mountHandler)
		mx.handle(mALL|mSTUB, pattern+"/", mountHandler)
		pattern += "/"
	}

	method := mALL
	subroutes, _ := handler.(Routes)
	if subroutes != nil {
		method |= mSTUB
	}
	n := mx.handle(method, pattern+"*", mountHandler)

	if subroutes != nil {
		n.subroutes = subroutes
	}
}

// Routes returns a slice of routing information from the tree, useful for traversing available routes of a router.
func (mx *Mux) Routes() []Route {
	return mx.tree.routes()
}

// Middlewares returns a slice of middleware handler functions.
func (mx *Mux) Middlewares() Middlewares {
	return mx.middlewares
}

// Match searches the routing tree for the handler matching the method/path.
// This is similar to routing an http request, but without executing the handler afterwards.
// The *Context state is updated  at runtime, so manage the state carefully or make a NewRouteContext().
func (mx *Mux) Match(rctx *Context, method, path string) bool {
	m, ok := methodMap[method]
	if !ok {
		return false
	}

	node, _, h := mx.tree.FindRoute(rctx, m, path)

	if node != nil && node.subroutes != nil {
		rctx.RoutePath = mx.nextRoutePath(rctx)
		return node.subroutes.Match(rctx, method, rctx.RoutePath)
	}

	return h != nil
}

func (mx *Mux) nextRoutePath(rctx *Context) string {
	routePath := "/"
	nx := len(rctx.routeParams.Keys) - 1 // index of last param in list
	if nx >= 0 && rctx.routeParams.Keys[nx] == "*" && len(rctx.routeParams.Values) > nx {
		routePath = "/" + rctx.routeParams.Values[nx]
	}
	return routePath
}

// handle registers http.Handler in the routing tree for a particular http method and routing pattern.
func (mx *Mux) handle(method methodType, pattern string, handler http.Handler) *node {
	if len(pattern) == 0 || pattern[0] != '/' {
		panic(fmt.Sprintf("gor: routing pattern must begin with '/' in '%s'", pattern))
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

// Recursively update data on child routers.
func (mx *Mux) updateSubRoutes(fn func(subMux *Mux)) {
	for _, r := range mx.tree.routes() {
		subMux, ok := r.SubRoutes.(*Mux)
		if !ok {
			continue
		}
		fn(subMux)
	}
}

// methodNotAllowedHandler is a helper function to respond with a 405, method not allowed.
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(405)
	_, err := w.Write(nil)
	if err != nil {
		golog.Error(err.Error())
	}
}
