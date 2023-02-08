package middleware

import (
	"context"
	"net/http"
	"time"
)

// Timeout is a middleware that cancels ctx after a given timeout and
// returns a 504 Gateway Timeout error to the client.
// It's required that you select the ctx.Done() channel to check
// the signal if the context has reached its deadline and return,
// otherwise the timeout signal will be just ignored.
func Timeout(timeout time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer func() {
				cancel()
				if ctx.Err() == context.DeadlineExceeded {
					w.WriteHeader(http.StatusGatewayTimeout)
				}
			}()

			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
