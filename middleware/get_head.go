package middleware

import (
	"net/http"

	"github.com/pchchv/gor"
)

// GetHead automatically route undefined HEAD requests to GET handlers.
func GetHead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			rctx := gor.RouteContext(r.Context())
			routePath := rctx.RoutePath
			if routePath == "" {
				if r.URL.RawPath != "" {
					routePath = r.URL.RawPath
				} else {
					routePath = r.URL.Path
				}
			}

			// temporary routing context to look-ahead before routing the request
			tctx := gor.NewRouteContext()

			// Try to find a HEAD handler for the routing path,
			// if it is not found, pass the router as through the GET route,
			// but make the request using the HEAD method.
			if !rctx.Routes.Match(tctx, "HEAD", routePath) {
				rctx.RouteMethod = "GET"
				rctx.RoutePath = routePath
				next.ServeHTTP(w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}
