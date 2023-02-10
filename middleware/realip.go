package middleware

import (
	"net"
	"net/http"
	"strings"
)

var (
	trueClientIP  = http.CanonicalHeaderKey("True-Client-IP")
	xForwardedFor = http.CanonicalHeaderKey("X-Forwarded-For")
	xRealIP       = http.CanonicalHeaderKey("X-Real-IP")
)

// RealIP is the middleware that sets the RemoteAddr http.Request to the parsing results of
// the True-Client-IP, X-Real-IP or X-Forwarded-For headers (in that order).
// This middleware must be inserted fairly early in the middleware stack to ensure
// that subsequent layers (such as request loggers) that check the RemoteAddr will see the intended value.
// You only need to use this middleware if you can trust the passed headers,
// for example because you have set up a reverse proxy, such as HAProxy or nginx, before gor.
// If your reverse proxies are configured to pass arbitrary header values from the client,
// or if you use this middleware without a reverse proxy,
// malicious clients will be able to make vulnerable to some attack.
func RealIP(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		if rip := realIP(r); rip != "" {
			r.RemoteAddr = rip
		}
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func realIP(r *http.Request) string {
	var ip string

	if tcip := r.Header.Get(trueClientIP); tcip != "" {
		ip = tcip
	} else if xrip := r.Header.Get(xRealIP); xrip != "" {
		ip = xrip
	} else if xff := r.Header.Get(xForwardedFor); xff != "" {
		i := strings.Index(xff, ",")
		if i == -1 {
			i = len(xff)
		}
		ip = xff[:i]
	}

	if ip == "" || net.ParseIP(ip) == nil {
		return ""
	}

	return ip
}
