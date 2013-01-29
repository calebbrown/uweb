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
//	"runtime"

	"fmt"
)



//////////////////////////////////////////////////////////////////////////////
// Helpers


func argIsContext(argType reflect.Type) bool {
	return argType.Kind() == reflect.Ptr &&
		argType.Elem() == reflect.TypeOf(Context{})
}

func argIsStringSlice(argType reflect.Type) bool {
	fmt.Println(argType)
	fmt.Println(argType.Kind())
	return argType.Kind() == reflect.Slice &&
		argType.Elem().Kind() == reflect.String
}


//////////////////////////////////////////////////////////////////////////////
// Response Handling

// responder is an internal interface that lets us pass around
//
type responseWriter interface {
	WriteResponse(http.ResponseWriter)
	StatusCode() int
}

// Response represents a http response to a received request
type Response struct {
	header http.Header
	Code int
	Content []byte
}

func NewResponse() *Response {
	r := &Response{
		Code: 200,
		header: make(http.Header),
	}
	return r
}

func NewRedirect(url string, code int) *Response {
	r := NewResponse()
	prettyUrl := html.EscapeString(url)
	if code == 0 {
		code = 302 // should this be 307?
	}
	// validate the status code is correct
	if code < 301 || code > 308 || code == 304 || code == 305 || code == 306 {
		panic("Invalid redirect status code supplied.")
	}
	r.Code = code
	r.header.Set("Location", url)
	r.Content = []byte("<html><body><a href=" + prettyUrl +
		">Redirecting to " + prettyUrl + "</a></body></html>")
	return r
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) StatusCode() int {
	return r.Code
}

func (r *Response) WriteResponse(w http.ResponseWriter) {
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

type ErrorResponse struct {
	Response
	Stack []byte // TODO: populate the stacktrace
	Message string
}

func NewError(code int, message string) *ErrorResponse {
	r := &ErrorResponse{
		Response: *NewResponse(),
	}
	r.Response.Code = code
	r.Content = []byte(message)
	return r
}


//////////////////////////////////////////////////////////////////////////////
// Context

// Context wraps up all the data related to the request and makes it easier to
// access it.
type Context struct {
	Request *http.Request
	Response *Response
	Get url.Values
	Method string
	Path string
	//Args []string
}

// Create a new instance of Context
func NewContext(r *http.Request) *Context {
	return &Context{
		Request: r,
		Response: NewResponse(),
		Get: r.URL.Query(),
		Path: r.URL.Path,
		Method: r.Method,
	}
}


//////////////////////////////////////////////////////////////////////////////
// Routing

/*
Target is a function that can process a request

The simplest target takes no inputs and has no outputs:

	func SimpleTarget() {
		Abort(404, "Page Not Found")
	}

A more complex target might take the Context and arguments parsed from the
url pattern and return a rendered string:

	func MyTarget(ctx *uweb.Context, arg1, arg2 string) string {
		ctx.Response.Header().Set("Content-Type", "text/plain")
		return fmt.Sprintf("arg1: %s, arg2: %s", arg1, arg2)
	}

	uweb.Get("^([0-9]+)/([a-z-]+)/", MyTarget)

Targets can also return any value that can be successfully converted into JSON
using json.Marshal.

	type MyStruct struct {
		Name string
	}

	func JSONTarget() MyStruct {
		return MyStruct{Name: "Joe Blogs"}
	}

*/
type Target interface {}

type route struct {
    re *regexp.Regexp
    targets map[string]Target
}

func newRoute(pattern string) (*route, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &route{
		re: re,
		targets: make(map[string]Target),
	}, nil
}

func (r *route) AddTarget(method string, target Target) {
	if method == "" {
		method = "ANY"
	}
	r.targets[strings.ToUpper(method)] = target
}

func (r *route) Parse(path string) []string {
	values := r.re.FindStringSubmatch(path)
	if len(values) == 0 {
		return nil
	}
	return values[1:]
}

func (r *route) TargetForMethod(method string) Target {
	method = strings.ToUpper(method)

	// target for method exists explicitly
	t, ok := r.targets[method]
	if ok {
		return t
	}
	// handle 'HEAD' if we have a "GET method"
	if method == "HEAD" {
		t, ok = r.targets["GET"]
		if ok {
			return t
		}
	}
	// if we can't find an explicit method target return the "ANY" target
	t, ok = r.targets["ANY"]
	if ok {
		return t
	}
	return nil
}

func (r *route) String() string {
	return fmt.Sprint(r.re)
}

type router struct {
	routes map[string]route
}

func newRouter() (*router) {
	return &router{routes: make(map[string]route)}
}

func (r *router) AddRoute(pattern, method string, target Target) error {
	route, ok := r.routes[pattern]
	if !ok {
		newRoute, err := newRoute(pattern)
		if err != nil {
			return err
		}
		r.routes[pattern] = *newRoute
		route = *newRoute
	}
	route.AddTarget(method, target)
	return nil
}

func (r *router) GetRoute(pattern string) (route, bool) {
	rt, ok := r.routes[pattern]
	return rt, ok
}

func (r *router) FindTarget(path, method string) (Target, []string) {
	var args []string
	var route route
	for _, route = range r.routes {
		args = route.Parse(path)
		if args != nil {
			break
		}
	}
	if args == nil {
		Abort(404, "Not Found")
	}
	target := route.TargetForMethod(method)
	if target == nil {
		Abort(405, "Method not allowed")
	}
	return target, args
}

func (r *route) StripPattern(path string) string {
	l := r.re.FindStringIndex(path)
	return path[l[1]:]
}



//////////////////////////////////////////////////////////////////////////////
// App


type Handler interface {
	Handle(ctx *Context) *Response
}

// An App is used to encapsulate a group of related routes.
type App struct {
     router router
}

// Creates a new empty App
func NewApp() *App {
	a := &App{}
	a.Reset()
	return a
}

func (a *App) addRoute(pattern, method string, target Target) error {
	return a.router.AddRoute(pattern, method, target)
}

// Map a function to a url pattern for any request method
func (a *App) Route(pattern string, target Target) error {
	return a.addRoute(pattern, "ANY", target)
}

// Map a function to a url pattern for DELETE requests
func (a *App) Delete(pattern string, target Target) error {
	return a.addRoute(pattern, "DELETE", target)
}

// Map a function to a url pattern for GET requests
func (a *App) Get(pattern string, target Target) error {
	return a.addRoute(pattern, "GET", target)
}

// Map a function to a url pattern for HEAD requests
func (a *App) Head(pattern string, target Target) error {
	return a.addRoute(pattern, "HEAD", target)
}

// Map a function to a url pattern for PATCH requests
func (a *App) Patch(pattern string, target Target) error {
	return a.addRoute(pattern, "PATCH", target)
}

// Map a function to a url pattern for POST requests
func (a *App) Post(pattern string, target Target) error {
	return a.addRoute(pattern, "POST", target)
}

// Map a function to a url pattern for PUT requests
func (a *App) Put(pattern string, target Target) error {
	return a.addRoute(pattern, "PUT", target)
}

// Map a function to a url pattern for OPTIONS requests
func (a *App) Options(pattern string, target Target) error {
	return a.addRoute(pattern, "OPTIONS", target)
}

func (a *App) Mount(pattern string, handler Handler) error {

	wrapper := func(ctx *Context) *Response {
		r, _ := a.router.GetRoute(pattern)
		ctx.Path = r.StripPattern(ctx.Path)
		return handler.Handle(ctx)
	}

	return a.addRoute(pattern, "ANY", wrapper)
}


// Resets the App back to it's initial state.
//
// This method will clear all the routes, mounts, error handlers, etc.
func (a *App) Reset() {
	a.router = *newRouter()
}


func (a *App) call(ctx *Context, target Target, args []string) []reflect.Value {
	var callArgs []reflect.Value
	function := reflect.ValueOf(target)

	funcType := function.Type()
	if argLength := funcType.NumIn(); argLength > 0 {
		argIndex := 0
		// Add the context the list of arguments if needed
		argType := funcType.In(argIndex)
		if argIsContext(argType) {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
			argIndex = 1
		}
		if argIndex < argLength {
			// Dump all the args as a []string if possible
			argType = funcType.In(argIndex)
			if argIsStringSlice(argType) {
				callArgs = append(callArgs, reflect.ValueOf(args))
			} else {
				// Otherwise append them one by one
				for _, arg := range args {
					callArgs = append(callArgs, reflect.ValueOf(arg))
				}
			}
		}
	}

	return function.Call(callArgs)
}

// find and call wraps up the process of path matching and calling the target
// so that we can capture any error responses that are generated for processing
//
// BUG(calebbrown): consider refactoring this process into one that is more
// suited to being used in a variety of contexts.
func (a *App) findAndCall(ctx *Context) (results []reflect.Value) {
	defer func() {
		if err := recover(); err != nil {
			results = make([]reflect.Value, 1)
			if response, ok := err.(*Response); ok {
				results[0] = reflect.ValueOf(response)
			} else if response, ok := err.(*ErrorResponse); ok {
				results[0] = reflect.ValueOf(response)
			} else {
				panic(err)
			}
		}
	}()

	target, args := a.router.FindTarget(ctx.Path, ctx.Method)

	return a.call(ctx, target, args)
}

// cast takes a return value from a target and attempts to convert it
// into something that can be used as a response.
func (a *App) cast(response *Response, results []reflect.Value) *Response {
	if len(results) == 0 {
		return response
	}
	if len(results) > 1 {
		panic("Too many values returned from target")
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
	case *ErrorResponse:
		r, _ := result.(*ErrorResponse)
		return &r.Response
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

func (a *App) Handle(ctx *Context) *Response {
	results := a.findAndCall(ctx)
	resp := a.cast(ctx.Response, results)
	return resp
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var resp responseWriter

	ctx := NewContext(r)
	ctx.Path = ctx.Path[1:] // remove the proceeding slash

	resp = a.Handle(ctx)
	if resp == nil {
		resp = NewError(404, "Page Not Found")
	}
	resp.WriteResponse(w)

	log(fmt.Sprintf("%s %s [%d]", r.Method, r.RequestURI, resp.StatusCode()))
}

func (a *App) Run(host string) {
	log("Listening on " + host)
	http.ListenAndServe(host, a)
}


// Default instance of App
var DefaultApp *App

var debugMode bool

func Route(pattern string, target Target) {
	DefaultApp.Route(pattern, target)
}

func Get(pattern string, target Target) {
	DefaultApp.Get(pattern, target)
}

func Head(pattern string, target Target) {
	DefaultApp.Head(pattern, target)
}

func Post(pattern string, target Target) {
	DefaultApp.Post(pattern, target)
}

func Put(pattern string, target Target) {
	DefaultApp.Put(pattern, target)
}

func Patch(pattern string, target Target) {
	DefaultApp.Patch(pattern, target)
}

func Delete(pattern string, target Target) {
	DefaultApp.Delete(pattern, target)
}

func Options(pattern string, target Target) {
	DefaultApp.Options(pattern, target)
}

func Mount(pattern string, handler Handler) error {
	return DefaultApp.Mount(pattern, handler)
}

func Run(host string) {
	DefaultApp.Run(host)
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
	r := NewRedirect(url, code)
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

// Abort breaks out of the current view and returns an
// error response
//
//    func MyView() string {
//        r, err := myFunc()
//        if err != nil {
//            Abort(503, "Oh no, an error occured!")
//        }
//        return r
//    }
func Abort(code int, message string) {
	r := NewError(code, message)
	panic(r)
}



// BUG(calebbrown): improve configurability

// BUG(calebbrown): capture errors in non-debug mode

// BUG(calebbrown): support Fast-CGI

// BUG(calebbrown): add more tests

// BUG(calebbrown): add error handler support

// BUG(calebbrown): add ability to merge two Apps together

// BUG(calebbrown): add ability to merge responses together
