package gor

import (
	"net/http"
	"regexp"
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
