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
