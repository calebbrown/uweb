// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"

	"fmt"
)

//////////////////////////////////////////////////////////////////////////////
// Helpers

func argIsContext(argType reflect.Type) bool {
	return argType.Kind() == reflect.Ptr &&
		argType.Elem() == reflect.TypeOf(Context{})
}

func argIsStringSlice(argType reflect.Type) bool {
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
	header  http.Header
	Code    int
	Content []byte
}

func NewResponse() *Response {
	r := &Response{
		Code:   200,
		header: make(http.Header),
	}
	r.Header().Set("Content-Type", "text/html; charset=utf-8")
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
			w.Header().Add(k, v)
		}
	}

	// write the headers
	w.WriteHeader(r.Code)

	// write the content
	w.Write(r.Content)
}

func (r *Response) Merge(resp *Response) {
	r.Code = resp.Code
	// TODO: Headers
}

type ErrorResponse struct {
	Response
	Stack   string
	Message string
}

func NewError(code int, message string) *ErrorResponse {
	r := &ErrorResponse{
		Response: *NewResponse(),
		Message:  message,
	}
	r.Response.Code = code
	r.Content = []byte(message)
	return r
}

func (e *ErrorResponse) SetStack(clean bool) {
	s := string(debug.Stack())

	// Strip the first 6 lines of the stack
	// This is a bit of a hack to clean up the stack trace.
	// Removes call to SetStack(), call to recover code, call in runtime
	if clean {
		for i := 0; i < 6; i++ {
			nlIndex := strings.IndexRune(s, '\n')
			if nlIndex < 0 {
				break
			}
			s = s[nlIndex+1:]
		}
	}

	e.Stack = s
}

//////////////////////////////////////////////////////////////////////////////
// Context

// Context wraps up all the data related to the request and makes it easier to
// access it.
type Context struct {
	Request  *http.Request
	Response *Response
	Get      url.Values
	Method   string
	Path     string
	//Args []string
}

// Create a new instance of Context
func NewContext(r *http.Request) *Context {
	return &Context{
		Request:  r,
		Response: NewResponse(),
		Get:      r.URL.Query(),
		Path:     r.URL.Path,
		Method:   r.Method,
	}
}

//////////////////////////////////////////////////////////////////////////////
// Callables: Targets and ErrorHandlers

/*
A Target is a function that can process a request. Targets are passed into the
methods Route, Get, Head, Post, etc.

	app.Get("^path/to/handle/", MyTarget)

	Route("^blog/([0-9)+)/edit/$", BlogEdit)

The simplest target has no inputs and no outputs:

	func SimpleTarget() {
		Abort(404, "Page Not Found")
	}

A more complex target might take the Context and args parsed from the
url pattern and return a rendered string:

	func MyTarget(ctx *uweb.Context, arg1, arg2 string) string {
		ctx.Response.Header().Set("Content-Type", "text/plain")
		return fmt.Sprintf("arg1: %s, arg2: %s", arg1, arg2)
	}

	uweb.Get("^([0-9]+)/([a-z-]+)/", MyTarget)


Additionally a target can be a variadic function, which is useful if
the target is called with an varing number of arguments:

	func MyTarget(args ...string) {
		...
	}

The return value can be of a variety of types: string, []byte, *Response, and
io.Reader are all supported.

Finally, a target can return a value of any type that can be successfully
converted into JSON using json.Marshal.

	type MyStruct struct {
		Name string
	}

	func JSONTarget() MyStruct {
		return MyStruct{Name: "Joe Blogs"}
	}

*/
type Target interface{}

type ErrorHandler interface{}

type wrappedTarget func(ctx *Context, args ...string) []reflect.Value
type wrappedErrorHandler func(ctx *Context, e *ErrorResponse) []reflect.Value

func wrapTarget(target Target) wrappedTarget {
	function := reflect.ValueOf(target)
	funcType := function.Type()
	hasContext := false
	hasArgs := false

	if inNum := funcType.NumIn(); inNum > 0 {
		firstArg := 0
		if argIsContext(funcType.In(0)) {
			hasContext = true
			firstArg = 1
		}
		hasArgs = inNum > firstArg
		if hasArgs {
			valid := true
			for i := firstArg; i < inNum; i++ {
				if funcType.In(i).Kind() != reflect.String {
					valid = false
					break
				}
			}

			if !(valid || argIsStringSlice(funcType.In(firstArg))) {
				panic(fmt.Sprintf("Invalid target function '%s'. Incorrect argument types.", function.String()))
			}
		}
	}

	var wrapped wrappedTarget = func(ctx *Context, args ...string) []reflect.Value {
		var callArgs []reflect.Value

		if hasContext {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
		}

		if hasArgs {
			for _, arg := range args {
				callArgs = append(callArgs, reflect.ValueOf(arg))
			}
		}

		return function.Call(callArgs)
	}

	return wrapped
}

func wrapErrorHandler(handler ErrorHandler) wrappedErrorHandler {
	function := reflect.ValueOf(handler)
	funcType := function.Type()
	hasContext := false
	hasResponse := false

	if inNum := funcType.NumIn(); inNum > 0 {
		in := 0
		if argIsContext(funcType.In(0)) {
			hasContext = true
			in = 1
		}
		hasResponse = inNum > in
		if hasResponse {
			inR := funcType.In(in)
			if !(inR.Kind() == reflect.Ptr &&
				inR.Elem() == reflect.TypeOf(ErrorResponse{})) {
				panic(fmt.Sprintf("Invalid error handler '%s'. Incorrect input types.", function.String()))
			}
		}
	}

	var wrapped wrappedErrorHandler = func(ctx *Context, e *ErrorResponse) []reflect.Value {
		var callArgs []reflect.Value

		if hasContext {
			callArgs = append(callArgs, reflect.ValueOf(ctx))
		}

		if hasResponse {
			callArgs = append(callArgs, reflect.ValueOf(e))
		}

		return function.Call(callArgs)
	}

	return wrapped
}

func defaultErrorHandler(ctx *Context, e *ErrorResponse) []reflect.Value {
	s := `<!DOCTYPE>
<html>
	<head>
		<title>Error: %d</title>
	</head>
	<body>
		<h1>%d %s</h1>
		%s
	</body>
</html>`
	detail := ""
	if Config.Debug && e.Stack != "" {
		detail = fmt.Sprintf("<div>%s</div><pre>%s</pre>", e.Message, e.Stack)
	}
	res := fmt.Sprintf(s, e.StatusCode(), e.StatusCode(), string(e.Content), detail)
	return []reflect.Value{reflect.ValueOf(res)}
}

//////////////////////////////////////////////////////////////////////////////
// Routing

type route struct {
	re      *regexp.Regexp
	targets map[string]wrappedTarget
}

func newRoute(pattern string) (*route, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &route{
		re:      re,
		targets: make(map[string]wrappedTarget),
	}, nil
}

func (r *route) AddTarget(method string, target wrappedTarget) {
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

func (r *route) TargetForMethod(method string) wrappedTarget {
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

func newRouter() *router {
	return &router{routes: make(map[string]route)}
}

func (r *router) AddRoute(pattern, method string, target wrappedTarget) error {
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

func (r *router) FindTarget(path, method string) (wrappedTarget, []string) {
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
	router        router
	errorHandlers map[int]wrappedErrorHandler
}

// Creates a new empty App
func NewApp() *App {
	a := &App{errorHandlers: make(map[int]wrappedErrorHandler)}
	a.Reset()
	return a
}

// addRoute takes a target and saves it in the router.
//
// It also wraps up the target in code that makes it easier to call
func (a *App) addRoute(pattern, method string, target Target) error {
	callable := wrapTarget(target)
	return a.router.AddRoute(pattern, method, callable)
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

// Register a handler to be called when an ErrorResponse is returned
func (a *App) Error(code int, handler ErrorHandler) {
	a.errorHandlers[code] = wrapErrorHandler(handler)
}

// Resets the App back to it's initial state.
//
// This method will clear all the routes, mounts, error handlers, etc.
func (a *App) Reset() {
	a.router = *newRouter()
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
				response := NewError(500, fmt.Sprint(err))
				response.Content = []byte("Internal Server Error")
				response.SetStack(true)
				results[0] = reflect.ValueOf(response)
			}
		}
	}()

	target, args := a.router.FindTarget(ctx.Path, ctx.Method)

	return target(ctx, args...)
}

// cast takes a return value from a target or error handler and attempts to
// convert it into something that can be used as a response.
func (a *App) cast(ctx *Context, results []reflect.Value) *Response {
	if len(results) == 0 {
		return ctx.Response
	}
	if len(results) > 1 {
		panic("Too many values returned from target")
	}
	result := results[0].Interface()

	// Try and convert simple known types
	switch result.(type) {
	case string:
		s, _ := result.(string)
		ctx.Response.Content = []byte(s)
	case []byte:
		bs, _ := result.([]byte)
		ctx.Response.Content = bs
	case *ErrorResponse:
		r, _ := result.(*ErrorResponse)
		ctx.Response.Merge(&r.Response)
		if handler, ok := a.errorHandlers[r.Code]; ok {
			return a.cast(ctx, handler(ctx, r))
		}
		return a.cast(ctx, defaultErrorHandler(ctx, r))
	case *Response:
		r, _ := result.(*Response)
		return r
	case io.Reader:
		r, _ := result.(io.Reader)
		var b bytes.Buffer
		b.ReadFrom(r)
		ctx.Response.Content = b.Bytes()
	default:
		// attempt to return a JSON data response
		json_content, err := json.Marshal(result)
		if err != nil {
			panic("Unknown response type returned from view function")
		}
		ctx.Response.Content = json_content
		ctx.Response.Header().Set("Content-Type", "application/json")
	}

	return ctx.Response
}

func (a *App) Handle(ctx *Context) *Response {
	results := a.findAndCall(ctx)
	resp := a.cast(ctx, results)
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

func (a *App) Run(host string) error {
	doAutoReload()
	log("Listening on " + host)
	return http.ListenAndServe(host, a)
}

// Default instance of App
var DefaultApp *App

// Configuration for µweb
//
// When Debug is set to true messages will be logged to stdout.
//
// When AutoReload is set to true, and Debug is set to true a call to Run()
// will wrap the execution up so that when a change is detected on a dependency
// it will restart the execution of the web application.
var Config struct {
	Debug      bool
	AutoReload bool
}

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

func Error(code int, handler ErrorHandler) {
	DefaultApp.Error(code, handler)
}

func Run(host string) error {
	return DefaultApp.Run(host)
}

func log(message string) {
	if Config.Debug {
		fmt.Printf("[muweb] %s\n", message)
	}
}

func doAutoReload() {
	if Config.Debug && Config.AutoReload {
		AutoReloader()
	}
}

func init() {
	DefaultApp = NewApp()
	Config.Debug = false
	Config.AutoReload = false
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

// BUG(calebbrown): add more tests - query and post data

// BUG(calebbrown): add ability to merge two Apps together

// BUG(calebbrown): add ability to merge responses together

// BUG(calebbrown): Form() and Query() methods in the context
