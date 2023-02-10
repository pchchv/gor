package middleware

import "net/http"

// Maybe middleware allows you to change the execution flow of the middleware stack depending on the return value of maybeFn(request).
// This is useful, for example, if you want to skip the middleware handler if the request does not satisfy the logic of maybeFn.
func Maybe(mw func(http.Handler) http.Handler, maybeFn func(r *http.Request) bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maybeFn(r) {
				mw(next).ServeHTTP(w, r)
			} else {
				next.ServeHTTP(w, r)
			}
		})
	}
}
