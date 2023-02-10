package middleware

import (
	"net/http"
	"strings"
)

type HeaderRouter map[string][]HeaderRoute

type HeaderRoute struct {
	Middleware func(next http.Handler) http.Handler
	MatchOne   Pattern
	MatchAny   []Pattern
}

type Pattern struct {
	prefix   string
	suffix   string
	wildcard bool
}

// RouteHeaders is a small header-based router that allows you to route
// a request flow through the middleware stack based on the request header.
// For example, suppose you want to configure multiple routers based on the Host request header.
func RouteHeaders() HeaderRouter {
	return HeaderRouter{}
}

func (hr HeaderRouter) Route(header, match string, middlewareHandler func(next http.Handler) http.Handler) HeaderRouter {
	header = strings.ToLower(header)
	k := hr[header]
	if k == nil {
		hr[header] = []HeaderRoute{}
	}

	hr[header] = append(hr[header], HeaderRoute{MatchOne: NewPattern(match), Middleware: middlewareHandler})
	return hr
}

func (hr HeaderRouter) RouteAny(header string, match []string, middlewareHandler func(next http.Handler) http.Handler) HeaderRouter {
	header = strings.ToLower(header)
	k := hr[header]
	if k == nil {
		hr[header] = []HeaderRoute{}
	}

	patterns := []Pattern{}

	for _, m := range match {
		patterns = append(patterns, NewPattern(m))
	}

	hr[header] = append(hr[header], HeaderRoute{MatchAny: patterns, Middleware: middlewareHandler})
	return hr
}

func (hr HeaderRouter) RouteDefault(handler func(next http.Handler) http.Handler) HeaderRouter {
	hr["*"] = []HeaderRoute{{Middleware: handler}}
	return hr
}

func (hr HeaderRouter) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(hr) == 0 {
			// skip if no routes set
			next.ServeHTTP(w, r)
		}

		// find first matching header route, and continue
		for header, matchers := range hr {
			headerValue := r.Header.Get(header)
			if headerValue == "" {
				continue
			}

			headerValue = strings.ToLower(headerValue)
			for _, matcher := range matchers {
				if matcher.IsMatch(headerValue) {
					matcher.Middleware(next).ServeHTTP(w, r)
					return
				}
			}
		}

		// if no match, check for "*" default route
		matcher, ok := hr["*"]
		if !ok || matcher[0].Middleware == nil {
			next.ServeHTTP(w, r)
			return
		}

		matcher[0].Middleware(next).ServeHTTP(w, r)
	})
}

func (r HeaderRoute) IsMatch(value string) bool {
	if len(r.MatchAny) > 0 {
		for _, m := range r.MatchAny {
			if m.Match(value) {
				return true
			}
		}
	} else if r.MatchOne.Match(value) {
		return true
	}
	return false
}

func NewPattern(value string) Pattern {
	p := Pattern{}

	if i := strings.IndexByte(value, '*'); i >= 0 {
		p.wildcard = true
		p.prefix = value[0:i]
		p.suffix = value[i+1:]
	} else {
		p.prefix = value
	}

	return p
}

func (p Pattern) Match(v string) bool {
	if !p.wildcard {
		if p.prefix == v {
			return true
		} else {
			return false
		}
	}

	return len(v) >= len(p.prefix+p.suffix) && strings.HasPrefix(v, p.prefix) && strings.HasSuffix(v, p.suffix)
}
