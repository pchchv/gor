package gor

import "context"

// Context is the default routing context set on the root node of the
// request context to track route patterns,
// URL parameters and optional routing path.
type Context struct {
	Routes Routes

	// parentCtx is the parent of this, for use Context as context.Context directly.
	// This is an optimization that saves 1 allocation.
	parentCtx context.Context

	// Routing path/method override used during route lookup.
	RoutePath   string
	RouteMethod string

	// URLParams are the stack of routeParams captured during the routing lifecycle in the sub-routers stack.
	URLParams RouteParams

	// Route parameters matched for the current sub-router.
	// It is intentionally not exported so that it cannot be tampered with.
	routeParams RouteParams

	// Endpoint routing pattern matching the request URI path or `RoutePath` of the current sub-router.
	// This value will be updated during the life cycle of the request that passes through the sub-routes stack.
	routePattern string

	// Routing pattern stack throughout the request lifecycle on all connected routers.
	// This is a record of all matching patterns in the sub-routers stack.
	RoutePatterns []string

	// methodNotAllowed hint
	methodNotAllowed bool
}

// contextKey is a value to be used with context.WithValue.
// It is used as a pointer, so it is placed in the interface{} without being selected.
type contextKey struct {
	name string
}

// RouteParams is a structure to track URL routing parameters efficiently.
type RouteParams struct {
	Keys   []string
	Values []string
}

// RouteCtxKey is the context.Context key to store the request context.
var RouteCtxKey = &contextKey{"RouteContext"}

// NewRouteContext returns a new routing Context object.
func NewRouteContext() *Context {
	return &Context{}
}

func (k *contextKey) String() string {
	return "chi context value " + k.name
}

// Add will append a URL parameter to the end of the route param
func (s *RouteParams) Add(key, value string) {
	s.Keys = append(s.Keys, key)
	s.Values = append(s.Values, value)
}

// Reset a routing context to its initial state.
func (ctx *Context) Reset() {
	ctx.Routes = nil
	ctx.RoutePath = ""
	ctx.RouteMethod = ""
	ctx.RoutePatterns = ctx.RoutePatterns[:0]
	ctx.URLParams.Keys = ctx.URLParams.Keys[:0]
	ctx.URLParams.Values = ctx.URLParams.Values[:0]
	ctx.routePattern = ""
	ctx.routeParams.Keys = ctx.routeParams.Keys[:0]
	ctx.routeParams.Values = ctx.routeParams.Values[:0]
	ctx.methodNotAllowed = false
	ctx.parentCtx = nil
}
