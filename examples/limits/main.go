package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/pchchv/gor"
	"github.com/pchchv/gor/middleware"
)

// This example demonstrates the use of Timeout, and Throttle middlewares.
// Timeout:	cancels request if processing takes longer than 2.5 seconds,
// server will respond with http.StatusGatewayTimeout.
// Throttle: limits the number of in-flight requests along a particular.
func main() {
	r := gor.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root."))
	})

	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	})

	// slow handlers.
	r.Group(func(r gor.Router) {
		// stop processing after 2.5 seconds.
		r.Use(middleware.Timeout(2500 * time.Millisecond))

		r.Get("/slow", func(w http.ResponseWriter, r *http.Request) {
			rand.Seed(time.Now().Unix())

			// processing will take 1-5 seconds.
			processTime := time.Duration(rand.Intn(4)+1) * time.Second

			select {
			case <-r.Context().Done():
				return

			case <-time.After(processTime):
				// the above channel simulates some hard work.
			}

			w.Write([]byte(fmt.Sprintf("Processed in %v seconds\n", processTime)))
		})
	})

	// Throttle very expensive handlers.
	r.Group(func(r gor.Router) {
		// stop processing after 30 seconds.
		r.Use(middleware.Timeout(30 * time.Second))

		// only one request will be processed at a time.
		r.Use(middleware.Throttle(1))

		r.Get("/throttled", func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
				switch r.Context().Err() {
				case context.DeadlineExceeded:
					w.WriteHeader(504)
					w.Write([]byte("Processing too slow\n"))
				default:
					w.Write([]byte("Canceled\n"))
				}
				return

			case <-time.After(5 * time.Second):
				// the above channel simulates some hard work.
			}

			w.Write([]byte("Processed\n"))
		})
	})

	http.ListenAndServe(":3333", r)
}
