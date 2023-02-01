package gor

import "net/http"

// Router consisting of basic routing methods, using only the standard net/http.
type Router interface {
	http.Handler

	// Use appends one or more middlewares to the Router stack.
	Use(middlewares ...func(http.Handler) http.Handler)

	// With built-in middleware modules to the endpoint handler.
	With(middlewares ...func(http.Handler) http.Handler) Router

	// Group adds a new inline Router along the current routing path,
	// with a fresh middleware stack for the inline Router.
	Group(fn func(r Router)) Router

	// Route mounts a sub Router on the `pattern` string.
	Route(pattern string, fn func(r Router)) Router

	// Mount attaches another http.Handler along the ./pattern/*
	Mount(pattern string, h http.Handler)

	// Handle adds routes for a `pattern` that matches all HTTP methods.
	Handle(pattern string, h http.Handler)
	// HandleFunc adds routes for a `pattern` that matches all HTTP methods.
	HandleFunc(pattern string, h http.HandlerFunc)

	// Method adds routes for `pattern` which matches the HTTP method `method`.
	Method(method, pattern string, h http.Handler)
	// MethodFunc adds routes for `pattern` which matches the HTTP method `method`.
	MethodFunc(method, pattern string, h http.HandlerFunc)

	// HTTP-method routing along `pattern`
	Get(pattern string, h http.HandlerFunc)
	Put(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Head(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Trace(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
	Connect(pattern string, h http.HandlerFunc)
	Options(pattern string, h http.HandlerFunc)

	// NotFound defines a handler that will respond whenever a route cannot be found.
	NotFound(h http.HandlerFunc)

	// MethodNotAllowed defines a handler that will react whenever the method is not allowed.
	MethodNotAllowed(h http.HandlerFunc)
}

// Middlewares is a slice of standard middleware handlers
// with methods to compose middleware chains and http.Handler's.
type Middlewares []func(http.Handler) http.Handler
