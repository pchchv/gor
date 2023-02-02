package gor

import (
	"fmt"
	"net/http"
	"regexp"
	"sort"
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

func (n *node) findPattern(pattern string) bool {
	nn := n
	for _, nds := range nn.child {
		if len(nds) == 0 {
			continue
		}

		n = nn.findEdge(nds[0].ntype, pattern[0])
		if n == nil {
			continue
		}

		var idx int
		var xpattern string

		switch n.ntype {
		case ntStatic:
			idx = longestPrefix(pattern, n.prefix)
			if idx < len(n.prefix) {
				continue
			}

		case ntParam, ntRegexp:
			idx = strings.IndexByte(pattern, '}') + 1

		case ntCatchAll:
			idx = longestPrefix(pattern, "*")

		default:
			panic("gor: unknown node type")
		}

		xpattern = pattern[idx:]
		if len(xpattern) == 0 {
			return true
		}

		return n.findPattern(xpattern)
	}
	return false
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

// Sort the list of nodes by label
func (ns nodes) Sort()              { sort.Sort(ns); ns.tailSort() }
func (ns nodes) Len() int           { return len(ns) }
func (ns nodes) Swap(i, j int)      { ns[i], ns[j] = ns[j], ns[i] }
func (ns nodes) Less(i, j int) bool { return ns[i].label < ns[j].label }

// tailSort pushes nodes with '/' as the tail to the end of the list for param nodes.
// The list order determines the traversal order.
func (ns nodes) tailSort() {
	for i := len(ns) - 1; i >= 0; i-- {
		if ns[i].ntype > ntStatic && ns[i].tail == '/' {
			ns.Swap(i, len(ns)-1)
			return
		}
	}
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

	// recurse on the children
	for _, ns := range n.child {
		for _, cn := range ns {
			if cn.walk(fn) {
				return true
			}
		}
	}
	return false
}

func (n *node) routes() []Route {
	rts := []Route{}

	n.walk(func(eps endpoints, subroutes Routes) bool {
		if eps[mSTUB] != nil && eps[mSTUB].handler != nil && subroutes == nil {
			return false
		}

		// group methodHandlers by unique patterns
		pats := make(map[string]endpoints)

		for mt, h := range eps {
			if h.pattern == "" {
				continue
			}
			p, ok := pats[h.pattern]
			if !ok {
				p = endpoints{}
				pats[h.pattern] = p
			}
			p[mt] = h
		}

		for p, mh := range pats {
			hs := make(map[string]http.Handler)
			if mh[mALL] != nil && mh[mALL].handler != nil {
				hs["*"] = mh[mALL].handler
			}

			for mt, h := range mh {
				if h.handler == nil {
					continue
				}
				m := methodTypeString(mt)
				if m == "" {
					continue
				}
				hs[m] = h.handler
			}

			rt := Route{subroutes, hs, p}
			rts = append(rts, rt)
		}

		return false
	})

	return rts
}

func (n *node) setEndpoint(method methodType, handler http.Handler, pattern string) {
	// Set the handler for the method type on the node
	if n.endpoints == nil {
		n.endpoints = make(endpoints)
	}

	paramKeys := patParamKeys(pattern)

	if method&mSTUB == mSTUB {
		n.endpoints.Value(mSTUB).handler = handler
	}
	if method&mALL == mALL {
		h := n.endpoints.Value(mALL)
		h.handler = handler
		h.pattern = pattern
		h.paramKeys = paramKeys
		for _, m := range methodMap {
			h := n.endpoints.Value(m)
			h.handler = handler
			h.pattern = pattern
			h.paramKeys = paramKeys
		}
	} else {
		h := n.endpoints.Value(method)
		h.handler = handler
		h.pattern = pattern
		h.paramKeys = paramKeys
	}
}

// addChild adds a new `child` node to the tree using `pattern` as the triplet key.
// For a URL router like the gor router, we separate the static, param, regexp and wildcard segments into different nodes.
// In addition, addChild will recursively call itself until each template segment is added to the url template tree as separate nodes, depending on type.
func (n *node) addChild(child *node, prefix string) *node {
	search := prefix

	// handler leaf node added to the tree is the child.
	// this may be overridden later down the flow
	hn := child

	// Parse next segment
	segType, _, segRexpat, segTail, segStartIdx, segEndIdx := patNextSegment(search)

	// Add child depending on next up segment
	switch segType {

	case ntStatic:
		// Search prefix is all static (that is, has no params in path)
		// noop

	default:
		// Search prefix contains a param, regexp or wildcard

		if segType == ntRegexp {
			rex, err := regexp.Compile(segRexpat)
			if err != nil {
				panic(fmt.Sprintf("gor: invalid regexp pattern '%s' in route param", segRexpat))
			}
			child.prefix = segRexpat
			child.rex = rex
		}

		if segStartIdx == 0 {
			// route starts with a param
			child.ntype = segType

			if segType == ntCatchAll {
				segStartIdx = -1
			} else {
				segStartIdx = segEndIdx
			}
			if segStartIdx < 0 {
				segStartIdx = len(search)
			}
			child.tail = segTail // for params, we set the tail

			if segStartIdx != len(search) {
				// add static edge for the remaining part, split the end.
				// its not possible to have adjacent param nodes, so its certainly
				// going to be a static node next.

				search = search[segStartIdx:] // advance search position

				nn := &node{
					ntype:  ntStatic,
					label:  search[0],
					prefix: search,
				}
				hn = child.addChild(nn, search)
			}

		} else if segStartIdx > 0 {
			// Route has some param

			// starts with a static segment
			child.ntype = ntStatic
			child.prefix = search[:segStartIdx]
			child.rex = nil

			// add the param edge node
			search = search[segStartIdx:]

			nn := &node{
				ntype: segType,
				label: search[0],
				tail:  segTail,
			}
			hn = child.addChild(nn, search)

		}
	}

	n.child[child.ntype] = append(n.child[child.ntype], child)
	n.child[child.ntype].Sort()
	return hn
}

func (s endpoints) Value(method methodType) *endpoint {
	mh, ok := s[method]
	if !ok {
		mh = &endpoint{}
		s[method] = mh
	}
	return mh
}

func methodTypeString(method methodType) string {
	for s, t := range methodMap {
		if method == t {
			return s
		}
	}
	return ""
}

// longestPrefix finds the length of the shared prefix of two strings
func longestPrefix(k1, k2 string) int {
	var i int
	max := len(k1)

	if l := len(k2); l < max {
		max = l
	}
	for i = 0; i < max; i++ {
		if k1[i] != k2[i] {
			break
		}
	}
	return i
}

// patNextSegment returns the next segment details from a pattern:
// node type, param key, regexp string, param tail byte, param starting index, param ending index
func patNextSegment(pattern string) (nodeType, string, string, byte, int, int) {
	ps := strings.Index(pattern, "{")
	ws := strings.Index(pattern, "*")

	if ps < 0 && ws < 0 {
		return ntStatic, "", "", 0, 0, len(pattern) // we return the entire thing
	}

	// Sanity check
	if ps >= 0 && ws >= 0 && ws < ps {
		panic("gor: wildcard '*' must be the last pattern in a route, otherwise use a '{param}'")
	}

	var tail byte = '/' // Default endpoint tail to / byte

	if ps >= 0 {
		// Param/Regexp pattern is next
		nt := ntParam

		// Read to closing } taking into account opens and closes in curl count (cc)
		cc := 0
		pe := ps
		for i, c := range pattern[ps:] {
			if c == '{' {
				cc++
			} else if c == '}' {
				cc--
				if cc == 0 {
					pe = ps + i
					break
				}
			}
		}
		if pe == ps {
			panic("gor: route param closing delimiter '}' is missing")
		}

		key := pattern[ps+1 : pe]
		pe++ // set end to next position

		if pe < len(pattern) {
			tail = pattern[pe]
		}

		var rexpat string
		if idx := strings.Index(key, ":"); idx >= 0 {
			nt = ntRegexp
			rexpat = key[idx+1:]
			key = key[:idx]
		}

		if len(rexpat) > 0 {
			if rexpat[0] != '^' {
				rexpat = "^" + rexpat
			}
			if rexpat[len(rexpat)-1] != '$' {
				rexpat += "$"
			}
		}

		return nt, key, rexpat, tail, ps, pe
	}

	// Wildcard pattern as finale
	if ws < len(pattern)-1 {
		panic("gor: wildcard '*' must be the last value in a route. trim trailing text or use a '{param}' instead")
	}
	return ntCatchAll, "*", "", 0, ws, len(pattern)
}

func patParamKeys(pattern string) []string {
	pat := pattern
	paramKeys := []string{}
	for {
		ptyp, paramKey, _, _, _, e := patNextSegment(pat)
		if ptyp == ntStatic {
			return paramKeys
		}
		for i := 0; i < len(paramKeys); i++ {
			if paramKeys[i] == paramKey {
				panic(fmt.Sprintf("gor: routing pattern '%s' contains duplicate param key, '%s'", pattern, paramKey))
			}
		}
		paramKeys = append(paramKeys, paramKey)
		pat = pat[e:]
	}
}
