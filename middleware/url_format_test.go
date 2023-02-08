package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pchchv/gor"
)

func TestURLFormat(t *testing.T) {
	r := gor.NewRouter()

	r.Use(URLFormat)

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("nothing here"))
	})

	r.Route("/samples/articles/samples.{articleID}", func(r gor.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			articleID := gor.URLParam(r, "articleID")
			w.Write([]byte(articleID))
		})
	})

	r.Route("/articles/{articleID}", func(r gor.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			articleID := gor.URLParam(r, "articleID")
			w.Write([]byte(articleID))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/articles/1.json", nil); resp != "1" {
		t.Fatalf(resp)
	}

	if _, resp := testRequest(t, ts, "GET", "/articles/1.xml", nil); resp != "1" {
		t.Fatalf(resp)
	}

	if _, resp := testRequest(t, ts, "GET", "/samples/articles/samples.1.json", nil); resp != "1" {
		t.Fatalf(resp)
	}

	if _, resp := testRequest(t, ts, "GET", "/samples/articles/samples.1.xml", nil); resp != "1" {
		t.Fatalf(resp)
	}
}

func TestURLFormatInSubRouter(t *testing.T) {
	r := gor.NewRouter()

	r.Route("/articles/{articleID}", func(r gor.Router) {
		r.Use(URLFormat)
		r.Get("/subroute", func(w http.ResponseWriter, r *http.Request) {
			articleID := gor.URLParam(r, "articleID")
			w.Write([]byte(articleID))
		})
	})

	ts := httptest.NewServer(r)
	defer ts.Close()

	if _, resp := testRequest(t, ts, "GET", "/articles/1/subroute.json", nil); resp != "1" {
		t.Fatalf(resp)
	}
}
