package middleware

import (
	"net/http"
	"path"

	"github.com/pchchv/gor"
)

// The CleanPath middleware will clean the user's request path from double-slash errors.
// For example, if a user requests /users//1 or //users////1, both requests will be treated as: /users/1
func CleanPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := gor.RouteContext(r.Context())

		routePath := rctx.RoutePath
		if routePath == "" {
			if r.URL.RawPath != "" {
				routePath = r.URL.RawPath
			} else {
				routePath = r.URL.Path
			}
			rctx.RoutePath = path.Clean(routePath)
		}
		next.ServeHTTP(w, r)
	})
}
