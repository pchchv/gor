package middleware

import (
	"net/http"
	"strings"
)

// AllowContentEncoding enforces the Content-Encoding whitelist of the request,
// otherwise it responds with a status of 415 Unsupported Media Type.
func AllowContentEncoding(contentEncoding ...string) func(next http.Handler) http.Handler {
	allowedEncodings := make(map[string]struct{}, len(contentEncoding))
	for _, encoding := range contentEncoding {
		allowedEncodings[strings.TrimSpace(strings.ToLower(encoding))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			requestEncodings := r.Header["Content-Encoding"]
			// skip checking for empty content body or no Content-Encoding
			if r.ContentLength == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// all encodings in the request must be allowed
			for _, encoding := range requestEncodings {
				if _, ok := allowedEncodings[strings.TrimSpace(strings.ToLower(encoding))]; !ok {
					w.WriteHeader(http.StatusUnsupportedMediaType)
					return
				}
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
