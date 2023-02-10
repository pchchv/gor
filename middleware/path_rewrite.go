package middleware

import (
	"net/http"
	"strings"
)

// PathRewrite is a simple middleware that allows to rewrite the path of a request URL.
func PathRewrite(old, new string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.Replace(r.URL.Path, old, new, 1)
			next.ServeHTTP(w, r)
		})
	}
}
