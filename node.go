package gor

import (
	"net/http"
	"regexp"
	"strings"
)

type node struct {
	// subroutes on the leaf node
	subroutes Routes

	// regexp matcher for regexp nodes
	rex *regexp.Regexp

	// HTTP handler endpoints on the leaf node
	endpoints endpoints

	// prefix is the common prefix we ignore
	prefix string

	// child nodes should be stored in-order for iteration, in groups of the node type
	child [ntCatchAll + 1]nodes

	// first byte of the child prefix
	tail byte

	// node type: static, regexp, param, catchAll
	ntype nodeType

	// first byte of the prefix
	label byte
}

type endpoint struct {
	// endpoint handler
	handler http.Handler

	// pattern is the routing pattern for handler nodes
	pattern string

	// parameter keys recorded on handler nodes
	paramKeys []string
}

// Route describes the details of a routing handler.
type Route struct {
	SubRoutes Routes
	Handlers  map[string]http.Handler // HTTP method
	Pattern   string
}

// endpoints is a mapping of http method constants to handlers for a given route.
type endpoints map[methodType]*endpoint

type nodeType uint8

type methodType uint

type nodes []*node

const (
	ntStatic   nodeType = iota // /home
	ntRegexp                   // /{id:[0-9]+}
	ntParam                    // /{user}
	ntCatchAll                 // /api/v1/*

	mSTUB methodType = 1 << iota
	mCONNECT
	mDELETE
	mGET
	mHEAD
	mOPTIONS
	mPATCH
	mPOST
	mPUT
	mTRACE
)

var (
	mALL = mCONNECT | mDELETE | mGET | mHEAD |
		mOPTIONS | mPATCH | mPOST | mPUT | mTRACE

	methodMap = map[string]methodType{
		http.MethodConnect: mCONNECT,
		http.MethodDelete:  mDELETE,
		http.MethodGet:     mGET,
		http.MethodHead:    mHEAD,
		http.MethodOptions: mOPTIONS,
		http.MethodPatch:   mPATCH,
		http.MethodPost:    mPOST,
		http.MethodPut:     mPUT,
		http.MethodTrace:   mTRACE,
	}
)

func (n *node) FindRoute(rctx *Context, method methodType, path string) (*node, endpoints, http.Handler) {
	// reset context routing pattern and params
	rctx.routePattern = ""
	rctx.routeParams.Keys = rctx.routeParams.Keys[:0]
	rctx.routeParams.Values = rctx.routeParams.Values[:0]

	// find routing handlers for the path
	rn := n.findRoute(rctx, method, path)
	if rn == nil {
		return nil, nil, nil
	}

	// record routing params in the request lifecycle
	rctx.URLParams.Keys = append(rctx.URLParams.Keys, rctx.routeParams.Keys...)
	rctx.URLParams.Values = append(rctx.URLParams.Values, rctx.routeParams.Values...)

	// record routing pattern in the request lifecycle
	if rn.endpoints[method].pattern != "" {
		rctx.routePattern = rn.endpoints[method].pattern
		rctx.RoutePatterns = append(rctx.RoutePatterns, rctx.routePattern)
	}

	return rn, rn.endpoints, rn.endpoints[method].handler
}

// Recursive traversal of edges, checking all nodeType groups along the path.
// This is similar to a multidimensional radix triplet search.
func (n *node) findRoute(rctx *Context, method methodType, path string) *node {
	nn := n
	search := path

	for t, nds := range nn.child {
		ntyp := nodeType(t)
		if len(nds) == 0 {
			continue
		}

		var xn *node
		xsearch := search

		var label byte
		if search != "" {
			label = search[0]
		}

		switch ntyp {
		case ntStatic:
			xn = nds.findEdge(label)
			if xn == nil || !strings.HasPrefix(xsearch, xn.prefix) {
				continue
			}
			xsearch = xsearch[len(xn.prefix):]

		case ntParam, ntRegexp:
			// short-circuit and return no matching route for empty param values
			if xsearch == "" {
				continue
			}

			// serially loop through each node grouped by the tail delimiter
			for idx := 0; idx < len(nds); idx++ {
				xn = nds[idx]

				// label for param nodes is the delimiter byte
				p := strings.IndexByte(xsearch, xn.tail)

				if p < 0 {
					if xn.tail == '/' {
						p = len(xsearch)
					} else {
						continue
					}
				} else if ntyp == ntRegexp && p == 0 {
					continue
				}

				if ntyp == ntRegexp && xn.rex != nil {
					if !xn.rex.MatchString(xsearch[:p]) {
						continue
					}
				} else if strings.IndexByte(xsearch[:p], '/') != -1 {
					// avoid a match across path segments
					continue
				}

				prevlen := len(rctx.routeParams.Values)
				rctx.routeParams.Values = append(rctx.routeParams.Values, xsearch[:p])
				xsearch = xsearch[p:]

				if len(xsearch) == 0 {
					if xn.isLeaf() {
						h := xn.endpoints[method]
						if h != nil && h.handler != nil {
							rctx.routeParams.Keys = append(rctx.routeParams.Keys, h.paramKeys...)
							return xn
						}

						// flag that the routing context found a route,
						// but not a corresponding supported method
						rctx.methodNotAllowed = true
					}
				}

				// recursively find the next node on this branch
				fin := xn.findRoute(rctx, method, xsearch)
				if fin != nil {
					return fin
				}

				// not found on this branch, reset vars
				rctx.routeParams.Values = rctx.routeParams.Values[:prevlen]
				xsearch = search
			}

			rctx.routeParams.Values = append(rctx.routeParams.Values, "")

		default:
			// catch-all nodes
			rctx.routeParams.Values = append(rctx.routeParams.Values, search)
			xn = nds[0]
			xsearch = ""
		}

		if xn == nil {
			continue
		}

		// did we find it yet?
		if len(xsearch) == 0 {
			if xn.isLeaf() {
				h := xn.endpoints[method]
				if h != nil && h.handler != nil {
					rctx.routeParams.Keys = append(rctx.routeParams.Keys, h.paramKeys...)
					return xn
				}

				// flag that the routing context found a route,
				// but not a corresponding supported method
				rctx.methodNotAllowed = true
			}
		}

		// recursively find the next node..
		fin := xn.findRoute(rctx, method, xsearch)
		if fin != nil {
			return fin
		}

		// Did not find final handler, let's remove the param here if it was set
		if xn.ntype > ntStatic {
			if len(rctx.routeParams.Values) > 0 {
				rctx.routeParams.Values = rctx.routeParams.Values[:len(rctx.routeParams.Values)-1]
			}
		}

	}

	return nil
}

func (n *node) findEdge(ntype nodeType, label byte) *node {
	nds := n.child[ntype]
	num := len(nds)
	idx := 0

	switch ntype {
	case ntStatic, ntParam, ntRegexp:
		i, j := 0, num-1
		for i <= j {
			idx = i + (j-i)/2
			if label > nds[idx].label {
				i = idx + 1
			} else if label < nds[idx].label {
				j = idx - 1
			} else {
				i = num // breaks cond
			}
		}
		if nds[idx].label != label {
			return nil
		}

		return nds[idx]
	default: // catch all
		return nds[idx]
	}
}

func (ns nodes) findEdge(label byte) *node {
	num := len(ns)
	idx := 0
	i, j := 0, num-1

	for i <= j {
		idx = i + (j-i)/2
		if label > ns[idx].label {
			i = idx + 1
		} else if label < ns[idx].label {
			j = idx - 1
		} else {
			i = num // breaks cond
		}
	}

	if ns[idx].label != label {
		return nil
	}

	return ns[idx]
}

func (n *node) isLeaf() bool {
	return n.endpoints != nil
}

func (n *node) getEdge(ntyp nodeType, label, tail byte, prefix string) *node {
	nds := n.child[ntyp]

	for i := 0; i < len(nds); i++ {
		if nds[i].label == label && nds[i].tail == tail {
			if ntyp == ntRegexp && nds[i].prefix != prefix {
				continue
			}
			return nds[i]
		}
	}
	return nil
}

func (n *node) walk(fn func(eps endpoints, subroutes Routes) bool) bool {
	if (n.endpoints != nil || n.subroutes != nil) && fn(n.endpoints, n.subroutes) {
		return true
	}

	// Recurse on the children
	for _, ns := range n.child {
		for _, cn := range ns {
			if cn.walk(fn) {
				return true
			}
		}
	}
	return false
}
