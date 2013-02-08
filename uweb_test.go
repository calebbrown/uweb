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

func simpleView6(ctx *uweb.Context, args []string) string {
	return args[0] + " " + args[1]
}

func simpleView7(ctx *uweb.Context, args ...string) string {
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

var app *uweb.App

func init() {
	app = uweb.NewApp()
	app.Route("^view1/$", simpleView1)
	app.Route("^view2/$", simpleView2)
	app.Route("^view3/$", simpleView3)
	app.Route("^view4/(world)/$", simpleView4)
	app.Route("^view5/(world)/$", simpleView5)
	app.Route("^view6/(hello)/(world)/$", simpleView6)
	app.Route("^view7/(hello)/(world)/$", simpleView7)
	app.Route("^notfound/$", notFoundView)
	app.Route("^redirect/$", redirectView)
	app.Route("^abort/$", abortView)

	app.Get("^method/$", func() string { return "get" })
	app.Head("^method/$", func() string { return "head" })
	app.Post("^method/$", func() string { return "post" })
	app.Put("^method/$", func() string { return "put" })
	app.Patch("^method/$", func() string { return "patch" })
	app.Delete("^method/$", func() string { return "delete" })
	app.Get("^method/get-only/$", func() string { return "get" })

	subApp := uweb.NewApp()
	app.Mount("^sub/", subApp)

	subApp.Get("^view/$", simpleView1)
}

func doSimpleRequest(method, url string, body io.Reader) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, url, body)
	out := httptest.NewRecorder()
	app.ServeHTTP(out, req)
	return out
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

	out7 := doSimpleRequest("GET", "/view7/hello/world/", nil)
	if out7.Body.String() != "hello world" {
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
		"head",
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
