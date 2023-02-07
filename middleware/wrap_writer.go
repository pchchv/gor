package middleware

import (
	"bufio"
	"io"
	"net"
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

type hijackWriter struct {
	basicWriter
}

type flushHijackWriter struct {
	basicWriter
}

// httpFancyWriter is a HTTP writer that additionally satisfies http.Flusher, http.Hijacker, and io.ReaderFrom.
// It exists for the common case of the http.ResponseWriter wrapper,
// which provides an http package to make the proxied object support the full set of proxied object methods.
type httpFancyWriter struct {
	basicWriter
}

// http2FancyWriter is an HTTP2 writer that additionally satisfies http.Flusher, and io.ReaderFrom.
// It exists for the common case of the http.ResponseWriter wrapper,
// which provides an http package to make the proxied object support the full set of proxied object methods.
type http2FancyWriter struct {
	basicWriter
}

// NewWrapResponseWriter wraps http.ResponseWriter, returning a proxy that allows to hook into various parts of the response process.
func NewWrapResponseWriter(w http.ResponseWriter, protoMajor int) WrapResponseWriter {
	_, fl := w.(http.Flusher)

	bw := basicWriter{ResponseWriter: w}

	if protoMajor == 2 {
		_, ps := w.(http.Pusher)
		if fl && ps {
			return &http2FancyWriter{bw}
		}
	} else {
		_, hj := w.(http.Hijacker)
		_, rf := w.(io.ReaderFrom)
		if fl && hj && rf {
			return &httpFancyWriter{bw}
		}
		if fl && hj {
			return &flushHijackWriter{bw}
		}
		if hj {
			return &hijackWriter{bw}
		}
	}

	if fl {
		return &flushWriter{bw}
	}

	return &bw
}

func (b *basicWriter) WriteHeader(code int) {
	if !b.wroteHeader {
		b.code = code
		b.wroteHeader = true
		b.ResponseWriter.WriteHeader(code)
	}
}

func (b *basicWriter) Write(buf []byte) (int, error) {
	b.maybeWriteHeader()
	n, err := b.ResponseWriter.Write(buf)

	if b.tee != nil {
		_, err2 := b.tee.Write(buf[:n])
		// prefer errors generated by the proxied writer.
		if err == nil {
			err = err2
		}
	}

	b.bytes += n

	return n, err
}

func (b *basicWriter) maybeWriteHeader() {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
}

func (b *basicWriter) Status() int {
	return b.code
}

func (b *basicWriter) BytesWritten() int {
	return b.bytes
}

func (b *basicWriter) Tee(w io.Writer) {
	b.tee = w
}

func (b *basicWriter) Unwrap() http.ResponseWriter {
	return b.ResponseWriter
}

func (f *flushWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &flushWriter{}

func (f *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

var _ http.Hijacker = &hijackWriter{}

func (f *flushHijackWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

func (f *flushHijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

var (
	_ http.Flusher  = &flushHijackWriter{}
	_ http.Hijacker = &flushHijackWriter{}
)

func (f *httpFancyWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

func (f *httpFancyWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj := f.basicWriter.ResponseWriter.(http.Hijacker)
	return hj.Hijack()
}

func (f *http2FancyWriter) Push(target string, opts *http.PushOptions) error {
	return f.basicWriter.ResponseWriter.(http.Pusher).Push(target, opts)
}

func (f *httpFancyWriter) ReadFrom(r io.Reader) (int64, error) {
	if f.basicWriter.tee != nil {
		n, err := io.Copy(&f.basicWriter, r)
		f.basicWriter.bytes += int(n)
		return n, err
	}
	rf := f.basicWriter.ResponseWriter.(io.ReaderFrom)
	f.basicWriter.maybeWriteHeader()
	n, err := rf.ReadFrom(r)
	f.basicWriter.bytes += int(n)
	return n, err
}

var (
	_ http.Flusher  = &httpFancyWriter{}
	_ http.Hijacker = &httpFancyWriter{}
	_ http.Pusher   = &http2FancyWriter{}
	_ io.ReaderFrom = &httpFancyWriter{}
)

func (f *http2FancyWriter) Flush() {
	f.wroteHeader = true
	fl := f.basicWriter.ResponseWriter.(http.Flusher)
	fl.Flush()
}

var _ http.Flusher = &http2FancyWriter{}
