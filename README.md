[![Go Reference](https://pkg.go.dev/badge/github.com/pchchv/gor.svg)](https://pkg.go.dev/github.com/pchchv/gor)

<div align="center">

# **_gor_**  

</div>

**`gor`** is a lightweight router for creating *Go HTTP services*. It helps build large *REST API services* that can be maintained as the project grows.  
**`gor`** is built on the [*context*](https://pkg.go.dev/context) package.  
*See [examples](https://github.com/pchchv/gor/blob/main/examples/)*

## Install

```sh
go get -u github.com/pchchv/gor
```


## Features

* ### **Fast**
* ### **Reliability**
* ### **Lightweight**
* ### **Context control**
* ### **Go.mod support**
* ### **100% compatible with net/http**
* ### **Designed for modular/composable APIs**


## Middlewares

gor comes with an optional `middleware` package that provides a set of standard `net/http` middleware.   
Any middleware in the ecosystem that is also compatible with `net/http` can be used with gor's mux.

### Core middlewares

------------------------------------------------------------------------------------------------------
| gor/middleware Handler | description                                                               |
| :--------------------- | :----------------------------------------------------------------------   |
| [AllowContentEncoding](https://pkg.go.dev/github.com/pchchv/gor/middleware#AllowContentEncoding) | Provides a white list of Content-Encoding headers of the request          |
| [AllowContentType](https://pkg.go.dev/github.com/pchchv/gor/middleware#AllowContentType)     | Explicit white list of accepted Content-Types requests                    |
| [BasicAuth](https://pkg.go.dev/github.com/pchchv/gor/middleware#BasicAuth)          | Basic HTTP authentication                                                 |
| [Compress](https://pkg.go.dev/github.com/pchchv/gor/middleware#Compress)          | Gzip compression for clients accepting compressed responses               |
| [ContentCharset](https://pkg.go.dev/github.com/pchchv/gor/middleware#ContentCharset)       | Providing encoding for Content-Type request headers                       |
| [CleanPath](https://pkg.go.dev/github.com/pchchv/gor/middleware#CleanPath)            | Clean the double slashes from request path                                |
| [GetHead](https://pkg.go.dev/github.com/pchchv/gor/middleware#GetHead)              | Automatically route undefined HEAD requests to GET handlers               |
| [Heartbeat](https://pkg.go.dev/github.com/pchchv/gor/middleware#Heartbeat)            | Monitoring endpoint to check the pulse of the servers                     |
| [Logger](https://pkg.go.dev/github.com/pchchv/gor/middleware#Logger)               | Logs the start and end of each request with the elapsed processing time   |
| [NoCache](https://pkg.go.dev/github.com/pchchv/gor/middleware#NoCache)              | Sets response headers to prevent caching by clients                       |
| [Profiler](https://pkg.go.dev/github.com/pchchv/gor/middleware#Profiler)             | Simple net/http/pprof connection to routers                               |
| [RealIP](https://pkg.go.dev/github.com/pchchv/gor/middleware#RealIP)              | Sets RemoteAddr http.Request to X-Real-IP or X-Forwarded-For              |
| [Recoverer](https://pkg.go.dev/github.com/pchchv/gor/middleware#Recoverer)            | Gracefully absorbs panic and prints a stack trace                         |
| [RequestID](https://pkg.go.dev/github.com/pchchv/gor/middleware#RequestID)            | Injects a request ID in the context of each request                       |
| [RedirectSlashes](https://pkg.go.dev/github.com/pchchv/gor/middleware#RedirectSlashes)      | Redirect slashes in routing paths                                         |
| [RouteHeaders](https://pkg.go.dev/github.com/pchchv/gor/middleware#RouteHeaders)         | Handling routes for request headers                                       |
| [SetHeader](https://pkg.go.dev/github.com/pchchv/gor/middleware#SetHeader)            | Middleware to set the key/response header value                           |
| [StripSlashes](https://pkg.go.dev/github.com/pchchv/gor/middleware#StripSlashes)         | Strip slashes in routing paths                                            |
| [Throttle](https://pkg.go.dev/github.com/pchchv/gor/middleware#Throttle)            | Puts a ceiling on the number of concurrent requests                       |
| [Timeout](https://pkg.go.dev/github.com/pchchv/gor/middleware#Timeout)              | Signals to the request context that the timeout deadline has been reached |
| [URLFormat](https://pkg.go.dev/github.com/pchchv/gor/middleware#URLFormat)            | Parse the extension from the url and put it in the request context        |
| [WithValue](https://pkg.go.dev/github.com/pchchv/gor/middleware#WithValue)           | Middleware to set the key/value in the context of a request               |
------------------------------------------------------------------------------------------------------

## context

[```context```](https://golang.org/pkg/context) is a tiny package available in stdlib since go1.7, providing a simple interface for context signaling via call stacks and goroutines.   
Learn more at [The Go Blog](https://blog.golang.org/context)