// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb_test

import (
	"github.com/calebbrown/uweb"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func simpleView1() string {
	return "hello world"
}

func simpleView2(ctx *uweb.Context) string {
	return "hello world"
}

func simpleView3(ctx *uweb.Context) {
	ctx.Response.Content = []byte("hello world")
}

func simpleView4(test string) string {
	return "hello " + test
}

func simpleView5(ctx *uweb.Context, test string) string {
	return "hello " + test
}

func simpleView6(ctx *uweb.Context, args ...string) string {
	return args[0] + " " + args[1]
}

func notFoundView() {
	uweb.Abort(404, "Page Not Found")
}

func redirectView() {
	uweb.Redirect("http://example.com/")
}

func abortView() {
	uweb.Abort(503, "the system is down")
}

func noAuthView() {
	uweb.Abort(401, "never seen")
}

func error401(r *uweb.ErrorResponse) *uweb.Response {
	new_r := uweb.NewResponse()
	new_r.Code = 999
	new_r.Content = []byte("not authed")
	return new_r
}

func cookieView(ctx *uweb.Context) string {
	cookie, err := ctx.GetCookie("test-cookie")
	if err == nil {
		return cookie
	}
	return "empty"
}

func cookieSet(ctx *uweb.Context) string {
	ctx.Response.SetCookie("test-key", "test-value")
	return "OK"
}

func cookieDelete(ctx *uweb.Context) string {
	ctx.Response.DeleteCookie("test-key")
	return "OK"
}

var app *uweb.App

func init() {
	app = uweb.NewApp()
	app.Route("^view1/$", simpleView1)
	app.Route("^view2/$", simpleView2)
	app.Route("^view3/$", simpleView3)
	app.Route("^view4/(world)/$", simpleView4)
	app.Route("^view5/(world)/$", simpleView5)
	app.Route("^view6/(hello)/(world)/$", simpleView6)
	app.Route("^notfound/$", notFoundView)
	app.Route("^redirect/$", redirectView)
	app.Route("^abort/$", abortView)
	app.Route("^noauth/$", noAuthView)
	app.Route("^cookie/$", cookieView)
	app.Route("^cookie/set/$", cookieSet)
	app.Route("^cookie/delete/$", cookieDelete)

	app.Get("^method/$", func() string { return "get" })
	app.Post("^method/$", func() string { return "post" })
	app.Put("^method/$", func() string { return "put" })
	app.Patch("^method/$", func() string { return "patch" })
	app.Delete("^method/$", func() string { return "delete" })
	app.Get("^method/get-only/$", func() string { return "get" })

	app.Head("^head1/$", func() string { return "test head" })
	app.Get("^head2/$", func() string { return "test get" })

	app.Error(401, error401)

	subApp := uweb.NewApp()
	app.Mount("^sub/", subApp)

	subApp.Get("^view/$", simpleView1)
}

func doRequest(req *http.Request) *httptest.ResponseRecorder {
	out := httptest.NewRecorder()
	app.ServeHTTP(out, req)
	return out
}

func doSimpleRequest(method, url string, body io.Reader) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, url, body)
	return doRequest(req)
}

func TestSimpleViews(t *testing.T) {
	out1 := doSimpleRequest("GET", "/view1/", nil)
	if out1.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}

	out2 := doSimpleRequest("GET", "/view2/", nil)
	if out2.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}

	out3 := doSimpleRequest("GET", "/view3/", nil)
	if out3.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}

	out4 := doSimpleRequest("GET", "/view4/world/", nil)
	if out4.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}

	out5 := doSimpleRequest("GET", "/view5/world/", nil)
	if out5.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}

	out6 := doSimpleRequest("GET", "/view6/hello/world/", nil)
	if out6.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}
}

func TestNotFound(t *testing.T) {
	out := doSimpleRequest("GET", "/notaview/", nil)
	if out.Code != 404 {
		t.Errorf("Status code %d != 404", out.Code)
	}
}

func TestNotFoundView(t *testing.T) {
	out := doSimpleRequest("GET", "/notfound/", nil)
	if out.Code != 404 {
		t.Errorf("Status code %d != 404", out.Code)
	}
}

func TestAbortView(t *testing.T) {
	out := doSimpleRequest("GET", "/abort/", nil)
	if out.Code != 503 {
		t.Errorf("Status code %d != 503", out.Code)
	}
}

func TestErrorHandlerView(t *testing.T) {
	out := doSimpleRequest("GET", "/noauth/", nil)
	if out.Code != 999 {
		t.Errorf("Status code %d != 999", out.Code)
	}
	if out.Body.String() != "not authed" {
		t.Error("Response content unexpected")
	}
}

func TestRedirectView(t *testing.T) {
	out := doSimpleRequest("GET", "/redirect/", nil)
	if out.Code != 302 {
		t.Errorf("Status code %d != 302", out.Code)
	}
	if loc := out.Header().Get("Location"); loc != "http://example.com/" {
		t.Errorf("Redirect returned unexpected location: %s", loc)
	}
}

func TestMethodTypes(t *testing.T) {
	methods := []string{
		"get",
		"post",
		"put",
		"patch",
		"delete",
	}
	for _, method := range methods {
		out := doSimpleRequest(method, "/method/", nil)
		content := out.Body.String()
		if content != method {
			t.Errorf("Method %s handled by view %s", method, content)
		}
	}
}

func TestHeadResponses(t *testing.T) {
	urls := []string{
		"/head1/",
		"/head2/",
	}
	for _, url := range urls {
		out := doSimpleRequest("head", url, nil)
		if len(out.Body.String()) != 0 {
			t.Error("HEAD request returned a Body")
		}
		if out.Header().Get("content-length") == "0" {
			t.Error("HEAD returned empty content-length.")
		}
	}
}

func TestSingleMethod(t *testing.T) {
	url := "/method/get-only/"

	out := doSimpleRequest("GET", url, nil)
	if out.Code != 200 {
		t.Errorf("Status code %d != 200", out.Code)
	}

	out = doSimpleRequest("POST", url, nil)
	if out.Code != 405 {
		t.Errorf("Status code %d != 405", out.Code)
	}

	out = doSimpleRequest("HEAD", url, nil)
	if out.Code != 200 {
		t.Errorf("Status code %d != 200", out.Code)
	}
}

func TestMountedApp(t *testing.T) {
	out1 := doSimpleRequest("GET", "/sub/view/", nil)
	if out1.Body.String() != "hello world" {
		t.Error("Unexpected body")
	}
}

func TestInvalidInputs(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("Expected a panic.")
		}
	}()
	// this will fail with a panic. these functions are invalid
	uweb.Route("^test_fail", func(foo int) int {
		return foo + 1
	})
}

func TestEmptyCookieView(t *testing.T) {
	out1 := doSimpleRequest("GET", "/cookie/", nil)
	if out1.Body.String() != "empty" {
		t.Error("Unexpected body")
	}
}

func TestCookieView(t *testing.T) {
	req, _ := http.NewRequest("GET", "/cookie/", nil)
	req.AddCookie(&http.Cookie{Name: "test-cookie1", Value: "test-cookie-value1"})
	req.AddCookie(&http.Cookie{Name: "test-cookie", Value: "test-cookie-value"})
	req.AddCookie(&http.Cookie{Name: "test-cookie2", Value: "test-cookie-value2"})
	out := doRequest(req)
	if out.Body.String() != "test-cookie-value" {
		t.Error("Unexpected body")
	}
}

func TestCookieSet(t *testing.T) {
	out := doSimpleRequest("GET", "/cookie/set/", nil)
	cookie := out.Header().Get("Set-Cookie")
	if cookie != "test-key=test-value; Path=/; HttpOnly" {
		t.Errorf("set-cookie header incorrect: %s", cookie)
	}
}

func TestCookieDelete(t *testing.T) {
	out := doSimpleRequest("GET", "/cookie/delete/", nil)
	cookie := out.Header().Get("Set-Cookie")
	if cookie != "test-key=; Path=/; Max-Age=0; HttpOnly" {
		t.Errorf("set-cookie header incorrect: %s", cookie)
	}
}
