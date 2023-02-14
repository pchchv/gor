package main

import (
	"net/http"

	"github.com/pchchv/gor"
	"github.com/pchchv/gor/middleware"
)

// This example demonstrates a project structure that
// defines a subrouter and its handlers on struct,
// and mounts them as subrouters to the parent router.
func main() {
	r := gor.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("."))
	})

	r.Mount("/users", usersResource{}.Routes())
	r.Mount("/todos", todosResource{}.Routes())

	http.ListenAndServe(":3333", r)
}
