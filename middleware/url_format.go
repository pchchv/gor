package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/pchchv/gor"
)

// URLFormatCtxKey is the context.Context key to store the URL format data for a request.
var URLFormatCtxKey = &contextKey{"URLFormat"}

// URLFormat is a middleware that parses the url extension from the request path and stores it
// in context as a string under the `middleware.URLFormatCtxKey`.
// The middleware trims the suffix from the routing path and continues routing.
// Routers must not include a url parameter for the suffix when using this middleware.
func URLFormat(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var format string
		ctx := r.Context()
		path := r.URL.Path
		rctx := gor.RouteContext(ctx)
		if rctx != nil && rctx.RoutePath != "" {
			path = rctx.RoutePath
		}

		if strings.Index(path, ".") > 0 {
			base := strings.LastIndex(path, "/")
			idx := strings.LastIndex(path[base:], ".")

			if idx > 0 {
				idx += base
				format = path[idx+1:]

				rctx.RoutePath = path[:idx]
			}
		}

		r = r.WithContext(context.WithValue(ctx, URLFormatCtxKey, format))

		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}
