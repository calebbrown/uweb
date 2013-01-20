// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb

import (
	"net/http"
	"net/url"
	"regexp"

	"fmt"
)


type Context struct {
	Request *http.Request
	Response http.ResponseWriter
	Get url.Values
	Args [][]string
}

func NewContext(r *http.Request, w http.ResponseWriter) *Context {
	return &Context{Request: r, Get: r.URL.Query(), Response: w}
}

func (c *Context) String() string {
	return fmt.Sprintf("Context: %s", c.Request.URL)
}




type Routable interface {
	Handle(path string, ctx *Context) bool
}



type routableFunc struct {
	Function func(ctx *Context)
}

func (rf *routableFunc) Handle(path string, ctx *Context) bool {
	rf.Function(ctx)
	return true
}



type route struct {
    re *regexp.Regexp
    Handler Routable
}

func NewRoute(pattern string, handler Routable) (*route, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &route{re: re, Handler: handler}, nil
}

func (r *route) Match(path string) bool {
	return r.re.MatchString(path)
}

func (r *route) Parse(path string) [][]string {
	names := r.re.SubexpNames()[1:]

	values := r.re.FindStringSubmatch(path)
	if len(values) == 0 {
		return nil
	}
	values = values[1:]

	args := make([][]string, len(values))
	for i, v := range values {
		args[i] = []string{names[i], v}
	}

	return args
}

func (r *route) StripPattern(path string) string {
	l := r.re.FindStringIndex(path)
	return path[l[1]:]
}


func (r *route) String() string {
	return fmt.Sprint(r.re)
}


// An App is used to encapsulate a group of related routes.
type App struct {
     routes []*route
}

func NewApp() *App {
	return &App{routes: nil}
}

func (a *App) Route(pattern string, function func(ctx *Context)) {
	a.Mount(pattern, &routableFunc{Function: function})
}
// func (a *App) Get(pattern string, function ...) {}
// func (a *App) Post(pattern string, function ...) {}
// func (a *App) Put(pattern string, function ...) {}
// func (a *App) Delete(pattern string, function ...) {}


func (a *App) Mount(pattern string, handler Routable) error {
	route, err := NewRoute(pattern, handler)
	if err != nil {
		return err
	}
	a.routes = append(a.routes, route)
	return nil
}

// func (a *App) Reset() {}

func (a *App) Handle(path string, ctx *Context) bool {
	found := false

	for _, route := range a.routes {
		parts := route.Parse(path)
		if parts != nil {
			new_path := route.StripPattern(path)
			found = route.Handler.Handle(new_path, ctx)
		}
		if found {
			break
		}
	}

	return found
}


func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r, w)

	found := a.Handle(r.URL.Path[1:], ctx)
	if !found {
		http.NotFound(w, r)
	}

	Log(r.Method + " " + r.RequestURI + " [STATUS]")
}


// Default instance of App
var DefaultApp *App

var debugMode bool

func Mount(pattern string, handler Routable) error {
	return DefaultApp.Mount(pattern, handler)
}

func Route(pattern string, function func(ctx *Context)) {
	DefaultApp.Route(pattern, function)
}

func RunApp(host string, app *App) {
	Log("Listening on " + host)
	http.ListenAndServe(host, app)
}

func Run(host string) {
	RunApp(host, DefaultApp)
}

// Debug is used to toggle debugging mode.
//
// In debugging mode information is logged to the console and errors aren't
// captured
func Debug(d bool) {
	debugMode = d
}

func Log(message string) {
	if debugMode {
		fmt.Printf("[muweb] %s\n", message)
	}
}

func init() {
	DefaultApp = NewApp()
	debugMode = false
}




//func hello(ctx *Context) {
//	fmt.Fprint(ctx.Response, "hello world")
//}


//func main() {
//	Debug(true)
//	app := NewApp()
//	app.Route("^bar/", hello)
//	Mount("^foo/", app)
//	Route("^foobar/", hello)
//	Run("localhost:4000")
//}

// BUG(calebbrown): capture errors in non-debug mode
// BUG(calebbrown): support Fast-CGI
// BUG(calebbrown): add tests

