# **gor examples**

* [hello_world](https://github.com/pchchv/gor/blob/main/examples/hello_world/main.go) — Hello World!
* [logging](https://github.com/pchchv/gor/blob/main/examples/logging/main.go) — Easy structured logging for any backend
* [fileserver](https://github.com/pchchv/gor/blob/main/examples/fileserver/main.go) — Easily serve static files
* [custom_handler](https://github.com/pchchv/gor/blob/main/examples/custom_handler/main.go) — Use a custom handler function signature
* [custom_method](https://github.com/pchchv/gor/blob/main/examples/custom_method/main.go) — Add a custom HTTP method
* [limits](https://github.com/pchchv/gor/blob/main/examples/limits/main.go) — Timeouts and Throttling
* [graceful](https://github.com/pchchv/gor/blob/main/examples/graceful/main.go) — Graceful context signaling and server shutdown
* [walk](https://github.com/pchchv/gor/blob/main/examples/walk/main.go) — Print to stdout a router's routes
* [todos_resource](https://github.com/pchchv/gor/blob/main/examples/todos_resource/main.go) — Struct routers/handlers, an example of another code layout style

##### Read `<example>/main.go` source to learn how service works and read comments for usage

## Usage

* `go run *.go` — note, example services run on port 3333
* Open another terminal and use curl to send some requests to your example service `curl -v http://localhost:3333/`