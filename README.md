# µweb

A micro web framework for [Go](http://golang.org).

[![Build Status](https://travis-ci.org/calebbrown/uweb.png?branch=master)](https://travis-ci.org/calebbrown/uweb)

## Overview

µweb is a small library designed to simplify the creation of web apps in Go.

It is heavily inspired by the Python library [BottlePy](http://bottlepy.org/).

**Note:** µweb is currently pre-alpha. The existing API is fairly stable but
may change.

## Example: Hello World

    package main

    import (
        "github.com/calebbrown/uweb"
        "fmt"
    )

    func main() {
        uweb.Route("^hello/(.*)$", func(name string) string {
            return fmt.Sprintf("<b>Hello, %s!</b>!", name)
        })
        uweb.Run("localhost:8080")
    }

Copy and paste this code into an editor, save the file as `hello.go`, in a shell run
`go run hello.go`, then point your browser to
[localhost:8080/hello/world](http://localhost:8080/hello/world).


## Installation

    go get github.com/calebbrown/uweb


## Documentation

Visit [http://godoc.org/github.com/calebbrown/uweb](http://godoc.org/github.com/calebbrown/uweb)
or simply run `godoc -http=":6060"` after installation and visit [localhost:6060/pkg/github.com/calebbrown/uweb](http://localhost:6060/pkg/github.com/calebbrown/uweb).

