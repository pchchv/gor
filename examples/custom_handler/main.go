package main

import (
	"errors"
	"net/http"

	"github.com/pchchv/gor"
)

type Handler func(w http.ResponseWriter, r *http.Request) error

func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		w.WriteHeader(503)
		w.Write([]byte("bad"))
	}
}

func customHandler(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query().Get("err")

	if q != "" {
		return errors.New(q)
	}

	w.Write([]byte("foo"))
	return nil
}

func main() {
	r := gor.NewRouter()
	r.Method("GET", "/", Handler(customHandler))
	http.ListenAndServe(":3333", r)
}
