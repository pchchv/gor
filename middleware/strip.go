package middleware

import (
	"fmt"
	"net/http"

	"github.com/pchchv/gor"
)

// StripSlashes is a middleware that will look for request paths with a slash at the end,
// remove it from the path and continue routing through the mux,
// if the route matches, it will serve the handler.
func StripSlashes(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var path string
		rctx := gor.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			path = rctx.RoutePath
		} else {
			path = r.URL.Path
		}
		if len(path) > 1 && path[len(path)-1] == '/' {
			newPath := path[:len(path)-1]
			if rctx == nil {
				r.URL.Path = newPath
			} else {
				rctx.RoutePath = newPath
			}
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

// RedirectSlashes is a middleware that will map request paths with
// a slash and redirect to the same path minus the slash.
func RedirectSlashes(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var path string
		rctx := gor.RouteContext(r.Context())
		if rctx != nil && rctx.RoutePath != "" {
			path = rctx.RoutePath
		} else {
			path = r.URL.Path
		}
		if len(path) > 1 && path[len(path)-1] == '/' {
			if r.URL.RawQuery != "" {
				path = fmt.Sprintf("%s?%s", path[:len(path)-1], r.URL.RawQuery)
			} else {
				path = path[:len(path)-1]
			}
			redirectURL := fmt.Sprintf("//%s%s", r.Host, path)
			http.Redirect(w, r, redirectURL, 301)
			return
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
