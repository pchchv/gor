package middleware

import "net/http"

// contextKey is a value to be used with context.WithValue.
// It is used as a pointer, so it is placed in the interface{} without allocation.
type contextKey struct {
	name string
}

// New will create a new middleware handler from a http.Handler.
func New(h http.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r)
		})
	}
}

func (k *contextKey) String() string {
	return "gor/middleware context value " + k.name
}
