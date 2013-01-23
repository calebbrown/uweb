// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb_test

import (
    "testing"
    "io"
    "net/http/httptest"
    "net/http"
    "uweb"
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

func notFoundView() {
    uweb.NotFound("Page Not Found")
}

func redirectView() {
    uweb.Redirect("http://example.com/")
}


var app *uweb.App

func init() {
    app = uweb.NewApp()
    app.Route("^view1/$", simpleView1)
    app.Route("^view2/$", simpleView2)
    app.Route("^view3/$", simpleView3)
    app.Route("^notfound/$", notFoundView)
    app.Route("^redirect/$", redirectView)

    app.Get("^method/$", func() string { return "get" })
    app.Post("^method/$", func() string { return "post" })
    app.Put("^method/$", func() string { return "put" })
    app.Patch("^method/$", func() string { return "patch" })
    app.Delete("^method/$", func() string { return "delete" })
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
}

func TestNotFoundView(t *testing.T) {
    out := doSimpleRequest("GET", "/notfound/", nil)
    if out.Code != 404 {
        t.Errorf("Status code %d != 404", out.Code)
    }
}

func TestRedirectView(t *testing.T) {
    out := doSimpleRequest("GET", "/redirect/", nil)
    if out.Code != 302 {
        t.Errorf("Status code %d != 302", out.Code)
    }
    if loc := out.Header().Get("Location") ; loc != "http://example.com/" {
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



