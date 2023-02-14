package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pchchv/gor"
	"github.com/pchchv/gor/middleware"
)

// FileServer conveniently sets up a http.FileServer handler
// to serve static files from a http.FileSystem.
func FileServer(r gor.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit any URL parameters.")
	}

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := gor.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}

func main() {
	r := gor.NewRouter()
	r.Use(middleware.Logger)

	// index handler
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hi"))
	})

	// create a route along /files that will serve contents from the ./data/ folder.
	workDir, _ := os.Getwd()
	filesDir := http.Dir(filepath.Join(workDir, "data"))
	FileServer(r, "/files", filesDir)

	http.ListenAndServe(":3333", r)
}
