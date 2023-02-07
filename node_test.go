package gor

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/pchchv/golog"
)

func TestTree(t *testing.T) {
	hStub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hIndex := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hFavicon := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleList := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleNear := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleShow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleShowRelated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleShowOpts := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleSlug := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hArticleByUser := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hUserList := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hUserShow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hAdminCatchall := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hAdminAppShow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hAdminAppShowCatchall := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hUserProfile := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hUserSuper := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hUserAll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hHubView1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hHubView2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hHubView3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	tr := &node{}

	tr.InsertRoute(mGET, "/", hIndex)
	tr.InsertRoute(mGET, "/favicon.ico", hFavicon)

	tr.InsertRoute(mGET, "/pages/*", hStub)

	tr.InsertRoute(mGET, "/article", hArticleList)
	tr.InsertRoute(mGET, "/article/", hArticleList)

	tr.InsertRoute(mGET, "/article/near", hArticleNear)
	tr.InsertRoute(mGET, "/article/{id}", hStub)
	tr.InsertRoute(mGET, "/article/{id}", hArticleShow)
	tr.InsertRoute(mGET, "/article/{id}", hArticleShow) // duplicate will have no effect
	tr.InsertRoute(mGET, "/article/@{user}", hArticleByUser)

	tr.InsertRoute(mGET, "/article/{sup}/{opts}", hArticleShowOpts)
	tr.InsertRoute(mGET, "/article/{id}/{opts}", hArticleShowOpts) // overwrite above route, latest wins

	tr.InsertRoute(mGET, "/article/{iffd}/edit", hStub)
	tr.InsertRoute(mGET, "/article/{id}//related", hArticleShowRelated)
	tr.InsertRoute(mGET, "/article/slug/{month}/-/{day}/{year}", hArticleSlug)

	tr.InsertRoute(mGET, "/admin/user", hUserList)
	tr.InsertRoute(mGET, "/admin/user/", hStub) // will get replaced by next route
	tr.InsertRoute(mGET, "/admin/user/", hUserList)

	tr.InsertRoute(mGET, "/admin/user//{id}", hUserShow)
	tr.InsertRoute(mGET, "/admin/user/{id}", hUserShow)

	tr.InsertRoute(mGET, "/admin/apps/{id}", hAdminAppShow)
	tr.InsertRoute(mGET, "/admin/apps/{id}/*", hAdminAppShowCatchall)

	tr.InsertRoute(mGET, "/admin/*", hStub) // catchall segment will get replaced by next route
	tr.InsertRoute(mGET, "/admin/*", hAdminCatchall)

	tr.InsertRoute(mGET, "/users/{userID}/profile", hUserProfile)
	tr.InsertRoute(mGET, "/users/super/*", hUserSuper)
	tr.InsertRoute(mGET, "/users/*", hUserAll)

	tr.InsertRoute(mGET, "/hubs/{hubID}/view", hHubView1)
	tr.InsertRoute(mGET, "/hubs/{hubID}/view/*", hHubView2)
	sr := NewRouter()
	sr.Get("/users", hHubView3)
	tr.InsertRoute(mGET, "/hubs/{hubID}/*", sr)
	tr.InsertRoute(mGET, "/hubs/{hubID}/users", hHubView3)

	tests := []struct {
		r string       // input request path
		h http.Handler // output matched handler
		k []string     // output param keys
		v []string     // output param values
	}{
		{r: "/", h: hIndex, k: []string{}, v: []string{}},
		{r: "/favicon.ico", h: hFavicon, k: []string{}, v: []string{}},

		{r: "/pages", h: nil, k: []string{}, v: []string{}},
		{r: "/pages/", h: hStub, k: []string{"*"}, v: []string{""}},
		{r: "/pages/yes", h: hStub, k: []string{"*"}, v: []string{"yes"}},

		{r: "/article", h: hArticleList, k: []string{}, v: []string{}},
		{r: "/article/", h: hArticleList, k: []string{}, v: []string{}},
		{r: "/article/near", h: hArticleNear, k: []string{}, v: []string{}},
		{r: "/article/neard", h: hArticleShow, k: []string{"id"}, v: []string{"neard"}},
		{r: "/article/123", h: hArticleShow, k: []string{"id"}, v: []string{"123"}},
		{r: "/article/123/456", h: hArticleShowOpts, k: []string{"id", "opts"}, v: []string{"123", "456"}},
		{r: "/article/@peter", h: hArticleByUser, k: []string{"user"}, v: []string{"peter"}},
		{r: "/article/22//related", h: hArticleShowRelated, k: []string{"id"}, v: []string{"22"}},
		{r: "/article/111/edit", h: hStub, k: []string{"iffd"}, v: []string{"111"}},
		{r: "/article/slug/sept/-/4/2015", h: hArticleSlug, k: []string{"month", "day", "year"}, v: []string{"sept", "4", "2015"}},
		{r: "/article/:id", h: hArticleShow, k: []string{"id"}, v: []string{":id"}},

		{r: "/admin/user", h: hUserList, k: []string{}, v: []string{}},
		{r: "/admin/user/", h: hUserList, k: []string{}, v: []string{}},
		{r: "/admin/user/1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/user//1", h: hUserShow, k: []string{"id"}, v: []string{"1"}},
		{r: "/admin/hi", h: hAdminCatchall, k: []string{"*"}, v: []string{"hi"}},
		{r: "/admin/lots/of/:fun", h: hAdminCatchall, k: []string{"*"}, v: []string{"lots/of/:fun"}},
		{r: "/admin/apps/333", h: hAdminAppShow, k: []string{"id"}, v: []string{"333"}},
		{r: "/admin/apps/333/woot", h: hAdminAppShowCatchall, k: []string{"id", "*"}, v: []string{"333", "woot"}},

		{r: "/hubs/123/view", h: hHubView1, k: []string{"hubID"}, v: []string{"123"}},
		{r: "/hubs/123/view/index.html", h: hHubView2, k: []string{"hubID", "*"}, v: []string{"123", "index.html"}},
		{r: "/hubs/123/users", h: hHubView3, k: []string{"hubID"}, v: []string{"123"}},

		{r: "/users/123/profile", h: hUserProfile, k: []string{"userID"}, v: []string{"123"}},
		{r: "/users/super/123/okay/yes", h: hUserSuper, k: []string{"*"}, v: []string{"123/okay/yes"}},
		{r: "/users/123/okay/yes", h: hUserAll, k: []string{"*"}, v: []string{"123/okay/yes"}},
	}

	for i, tt := range tests {
		rctx := NewRouteContext()

		_, handlers, _ := tr.FindRoute(rctx, mGET, tt.r)

		var handler http.Handler
		if methodHandler, ok := handlers[mGET]; ok {
			handler = methodHandler.handler
		}

		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}

		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}

		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func TestTreeMoar(t *testing.T) {
	hStub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub4 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub5 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub6 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub7 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub8 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub9 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub10 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub11 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub12 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub13 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub14 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub15 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	hStub16 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	tr := &node{}

	tr.InsertRoute(mGET, "/articlefun", hStub5)
	tr.InsertRoute(mGET, "/articles/{id}", hStub)
	tr.InsertRoute(mDELETE, "/articles/{slug}", hStub8)
	tr.InsertRoute(mGET, "/articles/search", hStub1)
	tr.InsertRoute(mGET, "/articles/{id}:delete", hStub8)
	tr.InsertRoute(mGET, "/articles/{iidd}!sup", hStub4)
	tr.InsertRoute(mGET, "/articles/{id}:{op}", hStub3)
	tr.InsertRoute(mGET, "/articles/{id}:{op}", hStub2)                              // this route sets a new handler for the above route
	tr.InsertRoute(mGET, "/articles/{slug:^[a-z]+}/posts", hStub)                    // up to tail '/' will only match if contents match the rex
	tr.InsertRoute(mGET, "/articles/{id}/posts/{pid}", hStub6)                       // /articles/123/posts/1
	tr.InsertRoute(mGET, "/articles/{id}/posts/{month}/{day}/{year}/{slug}", hStub7) // /articles/123/posts/09/04/1984/juice
	tr.InsertRoute(mGET, "/articles/{id}.json", hStub10)
	tr.InsertRoute(mGET, "/articles/{id}/data.json", hStub11)
	tr.InsertRoute(mGET, "/articles/files/{file}.{ext}", hStub12)
	tr.InsertRoute(mPUT, "/articles/me", hStub13)

	defer func() {
		if err := recover(); err == nil {
			t.Fatal("panic expected")
		}
	}()
	tr.InsertRoute(mGET, "/articles/{id}/{id}", hStub1) // panic expected, we're duplicating param keys

	tr.InsertRoute(mGET, "/pages/*", hStub)
	tr.InsertRoute(mGET, "/pages/*", hStub9)

	tr.InsertRoute(mGET, "/users/{id}", hStub14)
	tr.InsertRoute(mGET, "/users/{id}/settings/{key}", hStub15)
	tr.InsertRoute(mGET, "/users/{id}/settings/*", hStub16)

	tests := []struct {
		h http.Handler
		r string
		k []string
		v []string
		m methodType
	}{
		{m: mGET, r: "/articles/search", h: hStub1, k: []string{}, v: []string{}},
		{m: mGET, r: "/articlefun", h: hStub5, k: []string{}, v: []string{}},
		{m: mGET, r: "/articles/123", h: hStub, k: []string{"id"}, v: []string{"123"}},
		{m: mDELETE, r: "/articles/123mm", h: hStub8, k: []string{"slug"}, v: []string{"123mm"}},
		{m: mGET, r: "/articles/789:delete", h: hStub8, k: []string{"id"}, v: []string{"789"}},
		{m: mGET, r: "/articles/789!sup", h: hStub4, k: []string{"iidd"}, v: []string{"789"}},
		{m: mGET, r: "/articles/123:sync", h: hStub2, k: []string{"id", "op"}, v: []string{"123", "sync"}},
		{m: mGET, r: "/articles/456/posts/1", h: hStub6, k: []string{"id", "pid"}, v: []string{"456", "1"}},
		{m: mGET, r: "/articles/456/posts/09/04/1984/juice", h: hStub7, k: []string{"id", "month", "day", "year", "slug"}, v: []string{"456", "09", "04", "1984", "juice"}},
		{m: mGET, r: "/articles/456.json", h: hStub10, k: []string{"id"}, v: []string{"456"}},
		{m: mGET, r: "/articles/456/data.json", h: hStub11, k: []string{"id"}, v: []string{"456"}},

		{m: mGET, r: "/articles/files/file.zip", h: hStub12, k: []string{"file", "ext"}, v: []string{"file", "zip"}},
		{m: mGET, r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},
		{m: mGET, r: "/articles/files/photos.tar.gz", h: hStub12, k: []string{"file", "ext"}, v: []string{"photos", "tar.gz"}},

		{m: mPUT, r: "/articles/me", h: hStub13, k: []string{}, v: []string{}},
		{m: mGET, r: "/articles/me", h: hStub, k: []string{"id"}, v: []string{"me"}},
		{m: mGET, r: "/pages", h: nil, k: []string{}, v: []string{}},
		{m: mGET, r: "/pages/", h: hStub9, k: []string{"*"}, v: []string{""}},
		{m: mGET, r: "/pages/yes", h: hStub9, k: []string{"*"}, v: []string{"yes"}},

		{m: mGET, r: "/users/1", h: hStub14, k: []string{"id"}, v: []string{"1"}},
		{m: mGET, r: "/users/", h: nil, k: []string{}, v: []string{}},
		{m: mGET, r: "/users/2/settings/password", h: hStub15, k: []string{"id", "key"}, v: []string{"2", "password"}},
		{m: mGET, r: "/users/2/settings/", h: hStub16, k: []string{"id", "*"}, v: []string{"2", ""}},
	}

	for i, tt := range tests {
		var handler http.Handler
		rctx := NewRouteContext()
		paramKeys := rctx.routeParams.Keys
		paramValues := rctx.routeParams.Values
		_, handlers, _ := tr.FindRoute(rctx, tt.m, tt.r)

		if methodHandler, ok := handlers[tt.m]; ok {
			handler = methodHandler.handler
		}

		if fmt.Sprintf("%v", tt.h) != fmt.Sprintf("%v", handler) {
			t.Errorf("input [%d]: find '%s' expecting handler:%v , got:%v", i, tt.r, tt.h, handler)
		}

		if !stringSliceEqual(tt.k, paramKeys) {
			t.Errorf("input [%d]: find '%s' expecting paramKeys:(%d)%v , got:(%d)%v", i, tt.r, len(tt.k), tt.k, len(paramKeys), paramKeys)
		}

		if !stringSliceEqual(tt.v, paramValues) {
			t.Errorf("input [%d]: find '%s' expecting paramValues:(%d)%v , got:(%d)%v", i, tt.r, len(tt.v), tt.v, len(paramValues), paramValues)
		}
	}
}

func debugPrintTree(parent int, i int, n *node, label byte) bool {
	numEdges := 0
	for _, nds := range n.child {
		numEdges += len(nds)
	}

	if n.endpoints != nil {
		golog.Info("[node %d parent:%d] typ:%d prefix:%s label:%s tail:%s numEdges:%d isLeaf:%v handler:%v\n", i, parent, n.ntype, n.prefix, string(label), string(n.tail), numEdges, n.isLeaf(), n.endpoints)
	} else {
		golog.Info("[node %d parent:%d] typ:%d prefix:%s label:%s tail:%s numEdges:%d isLeaf:%v\n", i, parent, n.ntype, n.prefix, string(label), string(n.tail), numEdges, n.isLeaf())
	}
	parent = i
	for _, nds := range n.child {
		for _, e := range nds {
			i++
			if debugPrintTree(parent, i, e, e.label) {
				return true
			}
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if b[i] != a[i] {
			return false
		}
	}
	return true
}
