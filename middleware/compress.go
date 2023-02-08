package middleware

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

var defaultCompressibleContentTypes = []string{
	"text/html",
	"text/css",
	"text/plain",
	"text/javascript",
	"application/javascript",
	"application/x-javascript",
	"application/json",
	"application/atom+xml",
	"application/rss+xml",
	"image/svg+xml",
}

// Compressor represents a set of encoding configurations.
type Compressor struct {
	// The set of content types allowed to be compressed.
	allowedTypes     map[string]struct{}
	allowedWildcards map[string]struct{}

	encoders           map[string]EncoderFunc // The mapping of encoder names to encoder functions.
	pooledEncoders     map[string]*sync.Pool  // The mapping of pooled encoders to pools.
	encodingPrecedence []string               // The list of encoders in order of decreasing precedence.
	level              int                    // The compression level.
}

// EncoderFunc is a function that wraps the stream compression algorithm provided by io.Writer and returns it.
// If it fails, the function should return nil.
type EncoderFunc func(w io.Writer, level int) io.Writer

// Interface for types that allow resetting io.Writers.
type ioResetterWriter interface {
	io.Writer
	Reset(w io.Writer)
}

type compressResponseWriter struct {
	http.ResponseWriter

	// The streaming encoder writer to be used if there is one.
	// Otherwise, this is just the normal writer.
	w                io.Writer
	contentTypes     map[string]struct{}
	contentWildcards map[string]struct{}
	encoding         string
	wroteHeader      bool
	compressable     bool
}

type compressFlusher interface {
	Flush() error
}

// NewCompressor creates a new Compressor which will handle the encoding responses.
// The level must be one of those defined in the flate package.
// The types are the content types that are allowed to be compressed.
func NewCompressor(level int, types ...string) *Compressor {
	// If types are specified, set them as valid types.
	// If none are specified, use the default list.
	allowedTypes := make(map[string]struct{})
	allowedWildcards := make(map[string]struct{})

	if len(types) > 0 {
		for _, t := range types {
			if strings.Contains(strings.TrimSuffix(t, "/*"), "*") {
				panic(fmt.Sprintf("middleware/compress: Unsupported content-type wildcard pattern '%s'. Only '/*' supported", t))
			}
			if strings.HasSuffix(t, "/*") {
				allowedWildcards[strings.TrimSuffix(t, "/*")] = struct{}{}
			} else {
				allowedTypes[t] = struct{}{}
			}
		}
	} else {
		for _, t := range defaultCompressibleContentTypes {
			allowedTypes[t] = struct{}{}
		}
	}

	c := &Compressor{
		level:            level,
		encoders:         make(map[string]EncoderFunc),
		pooledEncoders:   make(map[string]*sync.Pool),
		allowedTypes:     allowedTypes,
		allowedWildcards: allowedWildcards,
	}

	// Set the default encoders.  The precedence order uses the reverse
	// ordering that the encoders were added. This means adding new encoders
	// will move them to the front of the order.
	c.SetEncoder("deflate", encoderDeflate)
	c.SetEncoder("gzip", encoderGzip)

	return c
}

// SetEncoder can be used to set the implementation of a compression algorithm.
func (c *Compressor) SetEncoder(encoding string, fn EncoderFunc) {
	encoding = strings.ToLower(encoding)
	if encoding == "" {
		panic("the encoding can not be empty")
	}
	if fn == nil {
		panic("attempted to set a nil encoder function")
	}

	// when adding a new encoder that is already registered, the encoder must first be cleared.
	if _, ok := c.pooledEncoders[encoding]; ok {
		delete(c.pooledEncoders, encoding)
	}
	if _, ok := c.encoders[encoding]; ok {
		delete(c.encoders, encoding)
	}

	// if the encoder supports Resetting (IoReseterWriter), then it can be pooled.
	encoder := fn(io.Discard, c.level)
	if encoder != nil {
		if _, ok := encoder.(ioResetterWriter); ok {
			pool := &sync.Pool{
				New: func() interface{} {
					return fn(io.Discard, c.level)
				},
			}
			c.pooledEncoders[encoding] = pool
		}
	}
	// if the encoder is not in the pooledEncoders, add it to the normal encoders.
	if _, ok := c.pooledEncoders[encoding]; !ok {
		c.encoders[encoding] = fn
	}

	for i, v := range c.encodingPrecedence {
		if v == encoding {
			c.encodingPrecedence = append(c.encodingPrecedence[:i], c.encodingPrecedence[i+1:]...)
		}
	}

	c.encodingPrecedence = append([]string{encoding}, c.encodingPrecedence...)
}

// Handler returns a new middleware that will compress the response based on the current Compressor.
func (c *Compressor) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		encoder, encoding, cleanup := c.selectEncoder(r.Header, w)

		cw := &compressResponseWriter{
			ResponseWriter:   w,
			w:                w,
			contentTypes:     c.allowedTypes,
			contentWildcards: c.allowedWildcards,
			encoding:         encoding,
			compressable:     false, // determined in post-handler
		}
		if encoder != nil {
			cw.w = encoder
		}
		// add the encoder to the pool if applicable.
		defer cleanup()
		defer cw.Close()

		next.ServeHTTP(cw, r)
	})
}

// selectEncoder returns the encoder, the name of the encoder, and a closer function.
func (c *Compressor) selectEncoder(h http.Header, w io.Writer) (io.Writer, string, func()) {
	header := h.Get("Accept-Encoding")

	// parse the names of all accepted algorithms from the header.
	accepted := strings.Split(strings.ToLower(header), ",")

	// find supported encoder by accepted list by precedence
	for _, name := range c.encodingPrecedence {
		if matchAcceptEncoding(accepted, name) {
			if pool, ok := c.pooledEncoders[name]; ok {
				encoder := pool.Get().(ioResetterWriter)
				cleanup := func() {
					pool.Put(encoder)
				}
				encoder.Reset(w)
				return encoder, name, cleanup

			}
			if fn, ok := c.encoders[name]; ok {
				return fn(w, c.level), name, func() {}
			}
		}
	}

	// no encoder found to match the accepted encoding
	return nil, "", func() {}
}

func (cw *compressResponseWriter) isCompressable() bool {
	// Parse the first part of the Content-Type response header.
	contentType := cw.Header().Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[0:idx]
	}

	// Is the content type compressable?
	if _, ok := cw.contentTypes[contentType]; ok {
		return true
	}
	if idx := strings.Index(contentType, "/"); idx > 0 {
		contentType = contentType[0:idx]
		_, ok := cw.contentWildcards[contentType]
		return ok
	}
	return false
}

func (cw *compressResponseWriter) WriteHeader(code int) {
	if cw.wroteHeader {
		cw.ResponseWriter.WriteHeader(code) // Allow multiple calls to propagate.
		return
	}
	cw.wroteHeader = true
	defer cw.ResponseWriter.WriteHeader(code)

	// Already compressed data?
	if cw.Header().Get("Content-Encoding") != "" {
		return
	}

	if !cw.isCompressable() {
		cw.compressable = false
		return
	}

	if cw.encoding != "" {
		cw.compressable = true
		cw.Header().Set("Content-Encoding", cw.encoding)
		cw.Header().Add("Vary", "Accept-Encoding")

		// The content-length after compression is unknown
		cw.Header().Del("Content-Length")
	}
}

func (cw *compressResponseWriter) Write(p []byte) (int, error) {
	if !cw.wroteHeader {
		cw.WriteHeader(http.StatusOK)
	}

	return cw.writer().Write(p)
}

func (cw *compressResponseWriter) writer() io.Writer {
	if cw.compressable {
		return cw.w
	} else {
		return cw.ResponseWriter
	}
}

func (cw *compressResponseWriter) Flush() {
	if f, ok := cw.writer().(http.Flusher); ok {
		f.Flush()
	}
	// If the underlying writer has a compression flush signature,
	// call this Flush() method instead
	if f, ok := cw.writer().(compressFlusher); ok {
		f.Flush()

		// Also flush the underlying response writer
		if f, ok := cw.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func (cw *compressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := cw.writer().(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, errors.New("chi/middleware: http.Hijacker is unavailable on the writer")
}

func (cw *compressResponseWriter) Push(target string, opts *http.PushOptions) error {
	if ps, ok := cw.writer().(http.Pusher); ok {
		return ps.Push(target, opts)
	}
	return errors.New("chi/middleware: http.Pusher is unavailable on the writer")
}

func (cw *compressResponseWriter) Close() error {
	if c, ok := cw.writer().(io.WriteCloser); ok {
		return c.Close()
	}
	return errors.New("chi/middleware: io.WriteCloser is unavailable on the writer")
}

// Compress is a middleware that compresses response body of a given content types to a data format based
// on Accept-Encoding request header. It uses a given compression level.
// NOTE: Be sure to set the Content-Type header on your response,
// otherwise this middleware will not compress the response body.
// For example, you must set w.Header().Set("Content-Type", http.DetectContentType(yourBody)) in the handler or set it manually.
// Passing a compression level of 5 is reasonable value.
func Compress(level int, types ...string) func(next http.Handler) http.Handler {
	compressor := NewCompressor(level, types...)
	return compressor.Handler
}

func encoderGzip(w io.Writer, level int) io.Writer {
	gw, err := gzip.NewWriterLevel(w, level)
	if err != nil {
		return nil
	}
	return gw
}

func encoderDeflate(w io.Writer, level int) io.Writer {
	dw, err := flate.NewWriter(w, level)
	if err != nil {
		return nil
	}
	return dw
}

func matchAcceptEncoding(accepted []string, encoding string) bool {
	for _, v := range accepted {
		if strings.Contains(v, encoding) {
			return true
		}
	}
	return false
}
