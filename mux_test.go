package gor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ctxKey struct {
	name string
}

func TestMuxBasic(t *testing.T) {
	var count uint64
	countermw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count++
			next.ServeHTTP(w, r)
		})
	}

	usermw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"user"}, "peter")
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	exmw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{"ex"}, "a")
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	logbuf := bytes.NewBufferString("")
	logmsg := "logmw test"
	logmw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logbuf.WriteString(logmsg)
			next.ServeHTTP(w, r)
		})
	}

	cxindex := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		user := ctx.Value(ctxKey{"user"}).(string)
		w.WriteHeader(200)
		_, err := w.Write([]byte(fmt.Sprintf("hi %s", user)))
		if err != nil {
			t.Error(err)
		}
	}

	ping := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("."))
		if err != nil {
			t.Error(err)
		}
	}

	headPing := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ping", "1")
		w.WriteHeader(200)
	}

	createPing := func(w http.ResponseWriter, r *http.Request) {
		// create ....
		w.WriteHeader(201)
	}

	pingAll := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("ping all"))
		if err != nil {
			t.Error(err)
		}
	}

	pingAll2 := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("ping all2"))
		if err != nil {
			t.Error(err)
		}
	}

	pingOne := func(w http.ResponseWriter, r *http.Request) {
		idParam := URLParam(r, "id")
		w.WriteHeader(200)
		_, err := w.Write([]byte(fmt.Sprintf("ping one id: %s", idParam)))
		if err != nil {
			t.Error(err)
		}
	}

	pingWoop := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("woop." + URLParam(r, "iidd")))
		if err != nil {
			t.Error(err)
		}
	}

	catchAll := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, err := w.Write([]byte("catchall"))
		if err != nil {
			t.Error(err)
		}
	}

	m := NewRouter()
	m.Use(countermw)
	m.Use(usermw)
	m.Use(exmw)
	m.Use(logmw)
	m.Get("/", cxindex)
	m.Method("GET", "/ping", http.HandlerFunc(ping))
	m.MethodFunc("GET", "/pingall", pingAll)
	m.MethodFunc("get", "/ping/all", pingAll)
	m.Get("/ping/all2", pingAll2)

	m.Head("/ping", headPing)
	m.Post("/ping", createPing)
	m.Get("/ping/{id}", pingWoop)
	m.Get("/ping/{id}", pingOne) // expected to overwrite to pingOne handler
	m.Get("/ping/{iidd}/woop", pingWoop)
	m.HandleFunc("/admin/*", catchAll)
	// m.Post("/admin/*", catchAll)

	ts := httptest.NewServer(m)
	defer ts.Close()

	// GET /
	if _, body := testRequest(t, ts, "GET", "/", nil); body != "hi peter" {
		t.Fatalf(body)
	}
	tlogmsg, _ := logbuf.ReadString(0)
	if tlogmsg != logmsg {
		t.Error("expecting log message from middleware:", logmsg)
	}

	// GET /ping
	if _, body := testRequest(t, ts, "GET", "/ping", nil); body != "." {
		t.Fatalf(body)
	}

	// GET /pingall
	if _, body := testRequest(t, ts, "GET", "/pingall", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all
	if _, body := testRequest(t, ts, "GET", "/ping/all", nil); body != "ping all" {
		t.Fatalf(body)
	}

	// GET /ping/all2
	if _, body := testRequest(t, ts, "GET", "/ping/all2", nil); body != "ping all2" {
		t.Fatalf(body)
	}

	// GET /ping/123
	if _, body := testRequest(t, ts, "GET", "/ping/123", nil); body != "ping one id: 123" {
		t.Fatalf(body)
	}

	// GET /ping/allan
	if _, body := testRequest(t, ts, "GET", "/ping/allan", nil); body != "ping one id: allan" {
		t.Fatalf(body)
	}

	// GET /ping/1/woop
	if _, body := testRequest(t, ts, "GET", "/ping/1/woop", nil); body != "woop.1" {
		t.Fatalf(body)
	}

	// HEAD /ping
	resp, err := http.Head(ts.URL + "/ping")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Error("head failed, should be 200")
	}
	if resp.Header.Get("X-Ping") == "" {
		t.Error("expecting X-Ping header")
	}

	// GET /admin/catch-this
	if _, body := testRequest(t, ts, "GET", "/admin/catch-thazzzzz", nil); body != "catchall" {
		t.Fatalf(body)
	}

	// POST /admin/catch-this
	resp, err = http.Post(ts.URL+"/admin/casdfsadfs", "text/plain", bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Error("POST failed, should be 200")
	}

	if string(body) != "catchall" {
		t.Error("expecting response body: 'catchall'")
	}

	// Custom http method DIE /ping/1/woop
	if resp, body := testRequest(t, ts, "DIE", "/ping/1/woop", nil); body != "" || resp.StatusCode != 405 {
		t.Fatalf(fmt.Sprintf("expecting 405 status and empty body, got %d '%s'", resp.StatusCode, body))
	}
}

func TestMuxMounts(t *testing.T) {
	r := NewRouter()

	r.Get("/{hash}", func(w http.ResponseWriter, r *http.Request) {
		v := URLParam(r, "hash")
		_, err := w.Write([]byte(fmt.Sprintf("/%s", v)))
		if err != nil {
			t.Error(err)
		}
	})

	r.Route("/{hash}/share", func(r Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			v := URLParam(r, "hash")
			_, err := w.Write([]byte(fmt.Sprintf("/%s/share", v)))
			if err != nil {
				t.Error(err)
			}
		})
		r.Get("/{network}", func(w http.ResponseWriter, r *http.Request) {
			v := URLParam(r, "hash")
			n := URLParam(r, "network")
			_, err := w.Write([]byte(fmt.Sprintf("/%s/share/%s", v, n)))
			if err != nil {
				t.Error(err)
			}
		})
	})

	m := NewRouter()
	m.Mount("/sharing", r)

	ts := httptest.NewServer(m)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/sharing/aBc", nil); body != "/aBc" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share", nil); body != "/aBc/share" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/sharing/aBc/share/twitter", nil); body != "/aBc/share/twitter" {
		t.Fatalf(body)
	}
}

func TestMuxPlain(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("bye"))
		if err != nil {
			t.Error(err)
		}
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, err := w.Write([]byte("nothing here"))
		if err != nil {
			t.Error(err)
		}
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}
	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "nothing here" {
		t.Fatalf(body)
	}
}

func TestMuxEmptyRoutes(t *testing.T) {
	mux := NewRouter()

	apiRouter := NewRouter()

	mux.Handle("/api*", apiRouter)

	if _, body := testHandler(t, mux, "GET", "/", nil); body != "404 page not found\n" {
		t.Fatalf(body)
	}

	if _, body := testHandler(t, apiRouter, "GET", "/", nil); body != "404 page not found\n" {
		t.Fatalf(body)
	}
}

// Test the mux that routes the slash.
func TestMuxTrailingSlash(t *testing.T) {
	r := NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		_, err := w.Write([]byte("nothing here"))
		if err != nil {
			t.Error(err)
		}
	})

	subRoutes := NewRouter()
	indexHandler := func(w http.ResponseWriter, r *http.Request) {
		accountID := URLParam(r, "accountID")
		_, err := w.Write([]byte(accountID))
		if err != nil {
			t.Error(err)
		}
	}
	subRoutes.Get("/", indexHandler)

	r.Mount("/accounts/{accountID}", subRoutes)
	r.Get("/accounts/{accountID}/", indexHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/accounts/admin", nil); body != "admin" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/accounts/admin/", nil); body != "admin" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "nothing here" {
		t.Fatalf(body)
	}
}

func TestMuxNestedNotFound(t *testing.T) {
	r := NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw"}, "mw"))
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})

	r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"with"}, "with"))
			next.ServeHTTP(w, r)
		})
	}).NotFound(func(w http.ResponseWriter, r *http.Request) {
		chkMw := r.Context().Value(ctxKey{"mw"}).(string)
		chkWith := r.Context().Value(ctxKey{"with"}).(string)
		w.WriteHeader(404)
		w.Write([]byte(fmt.Sprintf("root 404 %s %s", chkMw, chkWith)))
	})

	sr1 := NewRouter()

	sr1.Get("/sub", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub"))
	})
	sr1.Group(func(sr1 Router) {
		sr1.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r = r.WithContext(context.WithValue(r.Context(), ctxKey{"mw2"}, "mw2"))
				next.ServeHTTP(w, r)
			})
		})
		sr1.NotFound(func(w http.ResponseWriter, r *http.Request) {
			chkMw2 := r.Context().Value(ctxKey{"mw2"}).(string)
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("sub 404 %s", chkMw2)))
		})
	})

	sr2 := NewRouter()
	sr2.Get("/sub", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub2"))
	})

	r.Mount("/admin1", sr1)
	r.Mount("/admin2", sr2)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/nothing-here", nil); body != "root 404 mw with" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/admin1/sub", nil); body != "sub" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/admin1/nope", nil); body != "sub 404 mw2" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/admin2/sub", nil); body != "sub2" {
		t.Fatalf(body)
	}

	// not found pages should bubble up to the root.
	if _, body := testRequest(t, ts, "GET", "/admin2/nope", nil); body != "root 404 mw with" {
		t.Fatalf(body)
	}
}

func TestMuxNestedMethodNotAllowed(t *testing.T) {
	r := NewRouter()
	r.Get("/root", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("root 405"))
	})

	sr1 := NewRouter()
	sr1.Get("/sub1", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub1"))
	})
	sr1.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("sub1 405"))
	})

	sr2 := NewRouter()
	sr2.Get("/sub2", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("sub2"))
	})

	pathVar := NewRouter()
	pathVar.Get("/{var}", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pv"))
	})
	pathVar.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		w.Write([]byte("pv 405"))
	})

	r.Mount("/prefix1", sr1)
	r.Mount("/prefix2", sr2)
	r.Mount("/pathVar", pathVar)

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/root", nil); body != "root" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "PUT", "/root", nil); body != "root 405" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/prefix1/sub1", nil); body != "sub1" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "PUT", "/prefix1/sub1", nil); body != "sub1 405" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/prefix2/sub2", nil); body != "sub2" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "PUT", "/prefix2/sub2", nil); body != "root 405" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/pathVar/myvar", nil); body != "pv" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "DELETE", "/pathVar/myvar", nil); body != "pv 405" {
		t.Fatalf(body)
	}
}

func TestMuxComplicatedNotFound(t *testing.T) {
	decorateRouter := func(r *Mux) {
		// root router with groups
		r.Get("/auth", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("auth get"))
		})
		r.Route("/public", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("public get"))
			})
		})

		// sub router with groups
		sub0 := NewRouter()
		sub0.Route("/resource", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("private get"))
			})
		})
		r.Mount("/private", sub0)

		// sub router with groups
		sub1 := NewRouter()
		sub1.Route("/resource", func(r Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("private get"))
			})
		})
		r.With(func(next http.Handler) http.Handler { return next }).Mount("/private_mw", sub1)
	}

	testNotFound := func(t *testing.T, r *Mux) {
		ts := httptest.NewServer(r)
		defer ts.Close()

		// check that we didn't break correct routes
		if _, body := testRequest(t, ts, "GET", "/auth", nil); body != "auth get" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/public", nil); body != "public get" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/public/", nil); body != "public get" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/private/resource", nil); body != "private get" {
			t.Fatalf(body)
		}

		// check custom not-found on all levels
		if _, body := testRequest(t, ts, "GET", "/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/public/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/private/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/private/resource/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/private_mw/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		if _, body := testRequest(t, ts, "GET", "/private_mw/resource/nope", nil); body != "custom not-found" {
			t.Fatalf(body)
		}

		// check custom not-found on trailing slash routes
		if _, body := testRequest(t, ts, "GET", "/auth/", nil); body != "custom not-found" {
			t.Fatalf(body)
		}
	}

	t.Run("pre", func(t *testing.T) {
		r := NewRouter()
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("custom not-found"))
		})
		decorateRouter(r)
		testNotFound(t, r)
	})

	t.Run("post", func(t *testing.T) {
		r := NewRouter()
		decorateRouter(r)
		r.NotFound(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("custom not-found"))
		})
		testNotFound(t, r)
	})
}

func TestMuxWith(t *testing.T) {
	var (
		cmwInit1    uint64
		cmwInit2    uint64
		cmwHandler1 uint64
		cmwHandler2 uint64
	)
	mw1 := func(next http.Handler) http.Handler {
		cmwInit1++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cmwHandler1++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline1"}, "yes"))
			next.ServeHTTP(w, r)
		})
	}
	mw2 := func(next http.Handler) http.Handler {
		cmwInit2++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cmwHandler2++
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{"inline2"}, "yes"))
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bye"))
	})
	r.With(mw1).With(mw2).Get("/inline", func(w http.ResponseWriter, r *http.Request) {
		v1 := r.Context().Value(ctxKey{"inline1"}).(string)
		v2 := r.Context().Value(ctxKey{"inline2"}).(string)
		w.Write([]byte(fmt.Sprintf("inline %s %s", v1, v2)))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, body := testRequest(t, ts, "GET", "/hi", nil); body != "bye" {
		t.Fatalf(body)
	}

	if _, body := testRequest(t, ts, "GET", "/inline", nil); body != "inline yes yes" {
		t.Fatalf(body)
	}

	if cmwInit1 != 1 {
		t.Fatalf("expecting cmwInit1 to be 1, got %d", cmwInit1)
	}

	if cmwHandler1 != 1 {
		t.Fatalf("expecting cmwHandler1 to be 1, got %d", cmwHandler1)
	}

	if cmwInit2 != 1 {
		t.Fatalf("expecting cmwInit2 to be 1, got %d", cmwInit2)
	}

	if cmwHandler2 != 1 {
		t.Fatalf("expecting cmwHandler2 to be 1, got %d", cmwHandler2)
	}
}

func TestRouterFromMuxWith(t *testing.T) {
	t.Parallel()

	r := NewRouter()

	with := r.With(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})

	with.Get("/with_middleware", func(w http.ResponseWriter, r *http.Request) {})

	ts := httptest.NewServer(with)
	defer ts.Close()

	testRequest(t, ts, http.MethodGet, "/with_middleware", nil)
}

func TestMuxMiddlewareStack(t *testing.T) {
	var (
		stdmwInit      uint64
		ctxmwInit      uint64
		inCtxmwInit    uint64
		stdmwHandler   uint64
		ctxmwHandler   uint64
		inCtxmwHandler uint64
		handlerCount   uint64
		body           string
	)

	stdmw := func(next http.Handler) http.Handler {
		stdmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler++
			next.ServeHTTP(w, r)
		})
	}
	_ = stdmw

	ctxmw := func(next http.Handler) http.Handler {
		ctxmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctxmwHandler++
			ctx := r.Context()
			ctx = context.WithValue(ctx, ctxKey{"count.ctxmwHandler"}, ctxmwHandler)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}

	inCtxmw := func(next http.Handler) http.Handler {
		inCtxmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inCtxmwHandler++
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Use(stdmw)
	r.Use(ctxmw)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/ping" {
				w.Write([]byte("pong"))
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	r.With(inCtxmw).Get("/", func(w http.ResponseWriter, r *http.Request) {
		handlerCount++
		ctx := r.Context()
		ctxmwHandlerCount := ctx.Value(ctxKey{"count.ctxmwHandler"}).(uint64)
		w.Write([]byte(fmt.Sprintf("inits:%d reqs:%d ctxValue:%d", ctxmwInit, handlerCount, ctxmwHandlerCount)))
	})

	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("wooot"))
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	testRequest(t, ts, "GET", "/", nil)
	testRequest(t, ts, "GET", "/", nil)

	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "inits:1 reqs:3 ctxValue:3" {
		t.Fatalf("got: '%s'", body)
	}

	_, body = testRequest(t, ts, "GET", "/ping", nil)
	if body != "pong" {
		t.Fatalf("got: '%s'", body)
	}
}

func TestMuxRouteGroups(t *testing.T) {
	var (
		stdmwInit     uint64
		stdmwInit2    uint64
		stdmwHandler  uint64
		stdmwHandler2 uint64
		body          string
	)

	stdmw := func(next http.Handler) http.Handler {
		stdmwInit++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler++
			next.ServeHTTP(w, r)
		})
	}

	stdmw2 := func(next http.Handler) http.Handler {
		stdmwInit2++
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			stdmwHandler2++
			next.ServeHTTP(w, r)
		})
	}

	r := NewRouter()
	r.Group(func(r Router) {
		r.Use(stdmw)
		r.Get("/group", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("root group"))
		})
	})

	r.Group(func(r Router) {
		r.Use(stdmw2)
		r.Get("/group2", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("root group2"))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	_, body = testRequest(t, ts, "GET", "/group", nil)
	if body != "root group" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit != 1 || stdmwHandler != 1 {
		t.Logf("stdmw counters failed, should be 1:1, got %d:%d", stdmwInit, stdmwHandler)
	}

	_, body = testRequest(t, ts, "GET", "/group2", nil)
	if body != "root group2" {
		t.Fatalf("got: '%s'", body)
	}
	if stdmwInit2 != 1 || stdmwHandler2 != 1 {
		t.Fatalf("stdmw2 counters failed, should be 1:1, got %d:%d", stdmwInit2, stdmwHandler2)
	}
}

func TestMuxBig(t *testing.T) {
	var body, expected string
	r := bigMux()
	ts := httptest.NewServer(r)

	defer ts.Close()

	_, body = testRequest(t, ts, "GET", "/favicon.ico", nil)
	if body != "fav" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view", nil)
	if body != "/hubs/4/view reqid:1 session:anonymous" {
		t.Fatalf("got '%v'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/4/view/index.html", nil)
	if body != "/hubs/4/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "POST", "/hubs/ethereumhub/view/index.html", nil)
	if body != "/hubs/ethereumhub/view/index.html reqid:1 session:anonymous" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/", nil)
	if body != "/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/suggestions", nil)
	if body != "/suggestions reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/woot/444/hiiii", nil)
	if body != "/woot/444/hiiii" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123", nil)
	expected = "/hubs/123 reqid:1 session:elvis"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/touch", nil)
	if body != "/hubs/123/touch reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/webhooks", nil)
	if body != "/hubs/123/webhooks reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/posts", nil)
	if body != "/hubs/123/posts reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders", nil)
	if body != "404 page not found\n" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/", nil)
	if body != "/folders/ reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/public", nil)
	if body != "/folders/public reqid:1 session:elvis" {
		t.Fatalf("got '%s'", body)
	}
	_, body = testRequest(t, ts, "GET", "/folders/nothing", nil)
	if body != "404 page not found\n" {
		t.Fatalf("got '%s'", body)
	}
}

func TestMuxSubroutesBasic(t *testing.T) {
	var body, expected string
	r := NewRouter()
	hIndex := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("index"))
	})
	hArticlesList := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("articles-list"))
	})
	hSearchArticles := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("search-articles"))
	})
	hGetArticle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("get-article:%s", URLParam(r, "id"))))
	})
	hSyncArticle := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf("sync-article:%s", URLParam(r, "id"))))
	})

	r.Get("/", hIndex)
	r.Route("/articles", func(r Router) {
		r.Get("/", hArticlesList)
		r.Get("/search", hSearchArticles)
		r.Route("/{id}", func(r Router) {
			r.Get("/", hGetArticle)
			r.Get("/sync", hSyncArticle)
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	_, body = testRequest(t, ts, "GET", "/", nil)
	expected = "index"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	_, body = testRequest(t, ts, "GET", "/articles", nil)
	expected = "articles-list"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	_, body = testRequest(t, ts, "GET", "/articles/search", nil)
	expected = "search-articles"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	_, body = testRequest(t, ts, "GET", "/articles/123", nil)
	expected = "get-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	_, body = testRequest(t, ts, "GET", "/articles/123/sync", nil)
	expected = "sync-article:123"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

func TestMuxSubroutes(t *testing.T) {
	var body, expected string
	hHubView1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub1"))
	})
	hHubView2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub2"))
	})
	hHubView3 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub3"))
	})
	hAccountView1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("account1"))
	})
	hAccountView2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("account2"))
	})

	r := NewRouter()
	r.Get("/hubs/{hubID}/view", hHubView1)
	r.Get("/hubs/{hubID}/view/*", hHubView2)

	sr := NewRouter()
	sr.Get("/", hHubView3)
	r.Mount("/hubs/{hubID}/users", sr)
	r.Get("/hubs/{hubID}/users/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hub3 override"))
	})

	sr3 := NewRouter()
	sr3.Get("/", hAccountView1)
	sr3.Get("/hi", hAccountView2)

	r.Route("/accounts/{accountID}", func(r Router) {
		_ = r.(*Mux)
		r.Get("/", hAccountView1)
		r.Mount("/", sr3)
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	_, body = testRequest(t, ts, "GET", "/hubs/123/view", nil)
	expected = "hub1"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/view/index.html", nil)
	expected = "hub2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/users", nil)
	expected = "hub3"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/hubs/123/users/", nil)
	expected = "hub3 override"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44", nil)
	expected = "account1"
	if body != expected {
		t.Fatalf("request:%s expected:%s got:%s", "GET /accounts/44", expected, body)
	}
	_, body = testRequest(t, ts, "GET", "/accounts/44/hi", nil)
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	router := r
	req, _ := http.NewRequest("GET", "/accounts/44/hi", nil)

	rctx := NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	body = w.Body.String()
	expected = "account2"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}

	routePatterns := rctx.RoutePatterns
	if len(rctx.RoutePatterns) != 3 {
		t.Fatalf("expected 3 routing patterns, got:%d", len(rctx.RoutePatterns))
	}

	expected = "/accounts/{accountID}/*"
	if routePatterns[0] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[0])
	}

	expected = "/*"
	if routePatterns[1] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[1])
	}

	expected = "/hi"
	if routePatterns[2] != expected {
		t.Fatalf("routePattern, expected:%s got:%s", expected, routePatterns[2])
	}
}

func TestSingleHandler(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := URLParam(r, "name")
		w.Write([]byte("hi " + name))
	})

	r, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Error(err)
	}

	rctx := NewRouteContext()
	r = r.WithContext(context.WithValue(r.Context(), RouteCtxKey, rctx))
	rctx.URLParams.Add("name", "joe")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := w.Body.String()
	expected := "hi joe"
	if body != expected {
		t.Fatalf("expected:%s got:%s", expected, body)
	}
}

func TestServeHTTPExistingContext(t *testing.T) {
	r := NewRouter()
	r.Get("/hi", func(w http.ResponseWriter, r *http.Request) {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		w.Write([]byte(s))
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s, _ := r.Context().Value(ctxKey{"testCtx"}).(string)
		w.WriteHeader(404)
		w.Write([]byte(s))
	})

	testcases := []struct {
		Ctx            context.Context
		Method         string
		Path           string
		ExpectedBody   string
		ExpectedStatus int
	}{
		{
			Method:         "GET",
			Path:           "/hi",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "hi ctx"),
			ExpectedStatus: 200,
			ExpectedBody:   "hi ctx",
		},
		{
			Method:         "GET",
			Path:           "/hello",
			Ctx:            context.WithValue(context.Background(), ctxKey{"testCtx"}, "nothing here ctx"),
			ExpectedStatus: 404,
			ExpectedBody:   "nothing here ctx",
		},
	}

	for _, tc := range testcases {
		resp := httptest.NewRecorder()
		req, err := http.NewRequest(tc.Method, tc.Path, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}

		req = req.WithContext(tc.Ctx)
		r.ServeHTTP(resp, req)
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if resp.Code != tc.ExpectedStatus {
			t.Fatalf("%v != %v", tc.ExpectedStatus, resp.Code)
		}

		if string(b) != tc.ExpectedBody {
			t.Fatalf("%s != %s", tc.ExpectedBody, b)
		}
	}
}

func TestNestedGroups(t *testing.T) {
	handlerPrintCounter := func(w http.ResponseWriter, r *http.Request) {
		counter, _ := r.Context().Value(ctxKey{"counter"}).(int)
		w.Write([]byte(fmt.Sprintf("%v", counter)))
	}

	mwIncreaseCounter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			counter, _ := ctx.Value(ctxKey{"counter"}).(int)
			counter++
			ctx = context.WithValue(ctx, ctxKey{"counter"}, counter)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}

	// each route represents value of its counter.
	r := NewRouter() // counter == 0
	r.Get("/0", handlerPrintCounter)
	r.Group(func(r Router) {
		r.Use(mwIncreaseCounter) // counter == 1
		r.Get("/1", handlerPrintCounter)

		r.Handle("/2", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
		r.With(mwIncreaseCounter).Get("/2", handlerPrintCounter)

		r.Group(func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3
			r.Get("/3", handlerPrintCounter)
		})
		r.Route("/", func(r Router) {
			r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 3

			r.Handle("/4", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
			r.With(mwIncreaseCounter).Get("/4", handlerPrintCounter)

			r.Group(func(r Router) {
				r.Use(mwIncreaseCounter, mwIncreaseCounter) // counter == 5
				r.Get("/5", handlerPrintCounter)
				r.Handle("/6", Chain(mwIncreaseCounter).HandlerFunc(handlerPrintCounter))
				r.With(mwIncreaseCounter).Get("/6", handlerPrintCounter)
			})
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, route := range []string{"0", "1", "2", "3", "4", "5", "6"} {
		if _, body := testRequest(t, ts, "GET", "/"+route, nil); body != route {
			t.Errorf("expected %v, got %v", route, body)
		}
	}
}

func TestMiddlewarePanicOnLateUse(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello\n"))
	}

	mw := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()

	r.Get("/", handler)
	r.Use(mw)
}

func TestMountingExistingPath(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {}

	defer func() {
		if recover() == nil {
			t.Error("expected panic()")
		}
	}()

	r := NewRouter()

	r.Get("/", handler)
	r.Mount("/hi", http.HandlerFunc(handler))
	r.Mount("/hi", http.HandlerFunc(handler))
}

func testRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
		return nil, ""
	}
	defer resp.Body.Close()

	return resp, string(respBody)
}

func testHandler(t *testing.T, h http.Handler, method, path string, body io.Reader) (*http.Response, string) {
	r, _ := http.NewRequest(method, path, body)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	return w.Result(), w.Body.String()
}

func bigMux() Router {
	var (
		r   *Mux
		sr3 *Mux
		// sr1, sr2, sr3, sr4, sr5, sr6 *Mux
	)
	r = NewRouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{"requestID"}, "1")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "anonymous")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("fav"))
		})
		r.Get("/hubs/{hubID}/view", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view reqid:%s session:%s", URLParam(r, "hubID"),
				ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Get("/hubs/{hubID}/view/*", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubID"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Post("/hubs/{hubSlug}/view/*", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/hubs/%s/view/%s reqid:%s session:%s", URLParamFromCtx(ctx, "hubSlug"),
				URLParam(r, "*"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
	})
	r.Group(func(r Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), ctxKey{"session.user"}, "elvis")
				next.ServeHTTP(w, r.WithContext(ctx))
			})
		})
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/ reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})
		r.Get("/suggestions", func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			s := fmt.Sprintf("/suggestions reqid:%s session:%s", ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
			w.Write([]byte(s))
		})

		r.Get("/woot/{wootID}/*", func(w http.ResponseWriter, r *http.Request) {
			s := fmt.Sprintf("/woot/%s/%s", URLParam(r, "wootID"), URLParam(r, "*"))
			w.Write([]byte(s))
		})

		r.Route("/hubs", func(r Router) {
			_ = r.(*Mux) // sr1
			r.Route("/{hubID}", func(r Router) {
				_ = r.(*Mux) // sr2
				r.Get("/", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s reqid:%s session:%s",
						URLParam(r, "hubID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})
				r.Get("/touch", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/touch reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})

				sr3 = NewRouter()
				sr3.Get("/", func(w http.ResponseWriter, r *http.Request) {
					ctx := r.Context()
					s := fmt.Sprintf("/hubs/%s/webhooks reqid:%s session:%s", URLParam(r, "hubID"),
						ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
					w.Write([]byte(s))
				})
				sr3.Route("/{webhookID}", func(r Router) {
					_ = r.(*Mux) // sr4
					r.Get("/", func(w http.ResponseWriter, r *http.Request) {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/webhooks/%s reqid:%s session:%s", URLParam(r, "hubID"),
							URLParam(r, "webhookID"), ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
					})
				})

				r.Mount("/webhooks", Chain(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{"hook"}, true)))
					})
				}).Handler(sr3))

				r.Route("/posts", func(r Router) {
					_ = r.(*Mux) // sr5
					r.Get("/", func(w http.ResponseWriter, r *http.Request) {
						ctx := r.Context()
						s := fmt.Sprintf("/hubs/%s/posts reqid:%s session:%s", URLParam(r, "hubID"),
							ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
						w.Write([]byte(s))
					})
				})
			})
		})

		r.Route("/folders/", func(r Router) {
			_ = r.(*Mux) // sr6
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/ reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
			})
			r.Get("/public", func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				s := fmt.Sprintf("/folders/public reqid:%s session:%s",
					ctx.Value(ctxKey{"requestID"}), ctx.Value(ctxKey{"session.user"}))
				w.Write([]byte(s))
			})
		})
	})

	return r
}
