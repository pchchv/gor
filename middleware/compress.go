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
