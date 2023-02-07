package middleware

import (
	"io"
	"net/http"
)

// WrapResponseWriter is a proxy around an http.ResponseWriter
// that allows to hook into various parts of the response process.
type WrapResponseWriter interface {
	http.ResponseWriter

	// Status returns the HTTP status of the request, or 0 if one has not yet been sent.
	Status() int

	// BytesWritten returns the total number of bytes sent to the client.
	BytesWritten() int

	// Tee causes the response body to be written to the given io.Writer in
	// addition to proxying a write through it.
	// Only one io.Writer can be written to tee to at once:
	// installing a second one will overwrite the first one.
	// Before writing to a given io.Writer, the data will be sent to the proxy.
	// It is forbidden to change the proxy writer at the same time as writing.
	Tee(io.Writer)

	// Unwrap returns the original proxied target.
	Unwrap() http.ResponseWriter
}

// basicWriter wraps a http.ResponseWriter,
// which implements the minimal http.ResponseWriter interface.
type basicWriter struct {
	http.ResponseWriter
	wroteHeader bool
	code        int
	bytes       int
	tee         io.Writer
}

type flushWriter struct {
	basicWriter
}

func (f *flushWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &flushWriter{}
