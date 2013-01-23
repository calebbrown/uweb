# µweb

A micro web framework for [go](http://golang.org).

## Installation

    go get github.com/calebbrown/uweb


## Example

    package main

    import "github.com/calebbrown/uweb"

    func hello(name string) uweb.Response {
        return uweb.Ok("Hello, " + name)
    }

    func main() {
        uweb.Route("^(.*)$", hello)
        uweb.Run("localhost:6000")
    }


## Documentation

Simply run `godoc -http=":6060"`.

