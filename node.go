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
type endpoints map[methodTyp]*endpoint

type nodeType uint8

type methodTyp uint

type nodes []*node

const (
	ntStatic   nodeType = iota // /home
	ntRegexp                   // /{id:[0-9]+}
	ntParam                    // /{user}
	ntCatchAll                 // /api/v1/*

	mSTUB methodTyp = 1 << iota
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

	methodMap = map[string]methodTyp{
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
