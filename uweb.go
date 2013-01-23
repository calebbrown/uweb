// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb

import (
	"net/http"
	"net/url"
	"encoding/json"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"html"

	"fmt"
)


// Response represents a http response to a received request
type Response struct {
	header http.Header
	Code int
	Content []byte
}

func NewResponse() *Response {
	r := &Response{}
	r.Reset()
	return r
}

func (r *Response) Reset() {
	r.Code = 200
	r.Content = nil
	r.header = make(http.Header)
}

func (r *Response) NotFound(message string) {
	r.Reset()
	r.Code = 404
	r.Content = []byte(message)
}

func (r *Response) Error(message string) {
	r.Reset()
	r.Code = 503
	r.Content = []byte(message)
}

func (r *Response) Redirect(url string, code int) {
	prettyUrl := html.EscapeString(url)
	if code == 0 {
		code = 302 // should this be 307?
	}
	// validate the status code is correct
	if code < 301 || code > 308 || code == 304 || code == 305 || code == 306 {
		panic("Invalid redirect status code supplied.")
	}
	r.Reset()
	r.Code = code
	r.header.Set("Location", url)
	r.Content = []byte("<html><body><a href=" + prettyUrl +
		">Redirecting to " + prettyUrl + "</a></body></html>")
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) Output(w http.ResponseWriter) {
	r.Header().Set("Content-Length", strconv.Itoa(len(r.Content)))

	// set the headers
	for k, values := range r.header {
		for _, v := range values {
			w.Header().Add(k,v)
		}
	}

	// write the headers
	w.WriteHeader(r.Code)

	// write the content
	w.Write(r.Content)
}


// Context wraps up all the data related to the request and makes it easier to
// access it.
type Context struct {
	Request *http.Request
	Response *Response
	Get url.Values
	Args [][]string
}

// Create a new instance of Context
func NewContext(r *http.Request) *Context {
	return &Context{Request: r, Get: r.URL.Query(), Response: NewResponse()}
}


type Handler interface {
	Handle(path string, ctx *Context) *Response
}


// A view makes a function a Handler
type view struct {
	function reflect.Value
}

func newView(function interface{}) *view {
	return &view{function: reflect.ValueOf(function)}
}

func (v *view) cast(response *Response, results []reflect.Value) *Response {
	if len(results) == 0 {
		return response
	}
	if len(results) > 1 {
		panic("Too many values returned from view function")
	}
	result := results[0].Interface()

	switch result.(type) {
	case nil:
		break;
	case string:
		s, _ := result.(string)
		response.Content = []byte(s)
	case []byte:
		bs, _ := result.([]byte)
		response.Content = bs
	case *Response:
		r, _ := result.(*Response)
		return r
	default:
		// attempt to return a JSON data response
		json_content, err := json.Marshal(result)
		if err != nil {
			panic("Unknown response type returned from view function")
		}
		response.Content = json_content
		response.Header().Set("Content-Type", "application/json")
	}
	return response
}

func (v *view) Handle(path string, ctx *Context) (resp *Response) {
	defer func() {
		if err := recover(); err != nil {
			response, ok := err.(*Response);
			if !ok {
				panic(err)
			}
			resp = response
		}
	}()

	var args []reflect.Value
	funcType := v.function.Type()
	if funcType.NumIn() > 0 {
		argType := funcType.In(0)
		if argType.Kind() == reflect.Ptr &&
			argType.Elem() == reflect.TypeOf(Context{}) {
			args = append(args, reflect.ValueOf(ctx))
		}
	}
	result := v.function.Call(args)
	return v.cast(ctx.Response, result)
}



type route struct {
    re *regexp.Regexp
    Handler Handler
    method string // TODO
}

func newRoute(pattern, method string, handler Handler) (*route, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &route{
		re: re,
		method: strings.ToLower(method),
		Handler: handler,
	}, nil
}

func (r *route) MethodMatch(method string) bool {
	return r.method == "" || r.method == strings.ToLower(method)
}

func (r *route) Match(path, method string) bool {
	patternMatch := r.re.MatchString(path)
	return r.MethodMatch(method) && patternMatch
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

// Creates a new empty App
func NewApp() *App {
	a := &App{}
	a.Reset()
	return a
}

func (a *App) addRoute(pattern, method string, handler Handler) error {
	route, err := newRoute(pattern, method, handler)
	if err != nil {
		return err
	}
	a.routes = append(a.routes, route)
	return nil
}


// Map a function to a url pattern for any request method
func (a *App) Route(pattern string, function interface{}) error {
	return a.addRoute(pattern, "", newView(function))
}

// Map a function to a url pattern for DELETE requests
func (a *App) Delete(pattern string, function interface{}) error {
	return a.addRoute(pattern, "delete", newView(function))
}

// Map a function to a url pattern for GET requests
func (a *App) Get(pattern string, function interface{}) error {
	return a.addRoute(pattern, "get", newView(function))
}

// Map a function to a url pattern for HEAD requests
func (a *App) Head(pattern string, function interface{}) error {
	return a.addRoute(pattern, "head", newView(function))
}

// Map a function to a url pattern for PATCH requests
func (a *App) Patch(pattern string, function interface{}) error {
	return a.addRoute(pattern, "patch", newView(function))
}

// Map a function to a url pattern for POST requests
func (a *App) Post(pattern string, function interface{}) error {
	return a.addRoute(pattern, "post", newView(function))
}

// Map a function to a url pattern for PUT requests
func (a *App) Put(pattern string, function interface{}) error {
	return a.addRoute(pattern, "put", newView(function))
}

// Map a function to a url pattern for OPTIONS requests
func (a *App) Options(pattern string, function interface{}) error {
	return a.addRoute(pattern, "options", newView(function))
}

func (a *App) Mount(pattern string, handler Handler) error {
	return a.addRoute(pattern, "", handler)
}


// Resets the App back to it's initial state.
//
// This method will clear all the routes, mounts, error handlers, etc.
func (a *App) Reset() {
	a.routes = nil
}

func (a *App) Handle(path string, ctx *Context) *Response {
	var resp *Response

	for _, route := range a.routes {
		if !route.MethodMatch(ctx.Request.Method) {
			continue
		}
		parts := route.Parse(path)
		if parts != nil {
			new_path := route.StripPattern(path)
			resp = route.Handler.Handle(new_path, ctx)
		}
		if resp != nil {
			break
		}
	}

	return resp
}


func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r)

	resp := a.Handle(r.URL.Path[1:], ctx)
	if resp == nil {
		resp = NewResponse()
		resp.NotFound("Page Not Found")
	}
	resp.Output(w)

	log(fmt.Sprintf("%s %s [%d]", r.Method, r.RequestURI, resp.Code))
}


// Default instance of App
var DefaultApp *App

var debugMode bool

func Route(pattern string, function interface{}) {
	DefaultApp.Route(pattern, function)
}

func Get(pattern string, function interface{}) {
	DefaultApp.Get(pattern, function)
}

func Head(pattern string, function interface{}) {
	DefaultApp.Head(pattern, function)
}

func Post(pattern string, function interface{}) {
	DefaultApp.Post(pattern, function)
}

func Put(pattern string, function interface{}) {
	DefaultApp.Put(pattern, function)
}

func Patch(pattern string, function interface{}) {
	DefaultApp.Patch(pattern, function)
}

func Delete(pattern string, function interface{}) {
	DefaultApp.Delete(pattern, function)
}

func Options(pattern string, function interface{}) {
	DefaultApp.Options(pattern, function)
}


func Mount(pattern string, handler Handler) error {
	return DefaultApp.Mount(pattern, handler)
}


func RunApp(host string, app *App) {
	log("Listening on " + host)
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

func log(message string) {
	if debugMode {
		fmt.Printf("[muweb] %s\n", message)
	}
}

func init() {
	DefaultApp = NewApp()
	debugMode = false
}

// RedirectWithCode behaves like Redirect, but allows a custom HTTP
// status code to be supplied.
//
// The status code must be a valid redirect code (301, 302, 303, 307, 308)
func RedirectWithCode(url string, code int) {
	r := NewResponse()
	r.Redirect(url, code)
	panic(r)
}

// Redirect breaks out of the current view and returns a
// redirect (302) response, redirecting the User-Agent to
// the specified URL
//
//    func MyView() {
//        Redirect("http://example.com/")
//    }
func Redirect(url string) {
	RedirectWithCode(url, 302)
}

// NotFound breaks out of the current view and returns a
// not found (404) response
//
//    func MyView() {
//        NotFound("Nothing to see here...")
//    }
func NotFound(message string) {
	r := NewResponse()
	r.NotFound(message)
	panic(r)
}

// Abort breaks out of the current view and returns a
// server error (503) response
//
//    func MyView() string {
//        r, err := myFunc()
//        if err != nil {
//            Abort("Oh no, an error occured!")
//        }
//        return r
//    }
func Abort(message string) {
	r := NewResponse()
	r.Error(message)
	panic(r)
}



// BUG(calebbrown): improve configurability

// BUG(calebbrown): capture errors in non-debug mode

// BUG(calebbrown): support Fast-CGI

// BUG(calebbrown): add more tests

