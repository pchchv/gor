package middleware

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
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
	encoder := fn(ioutil.Discard, c.level)
	if encoder != nil {
		if _, ok := encoder.(ioResetterWriter); ok {
			pool := &sync.Pool{
				New: func() interface{} {
					return fn(ioutil.Discard, c.level)
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
