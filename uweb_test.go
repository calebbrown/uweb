// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb

import (
    "testing"
    "fmt"
)

func viewFunc(ctx *Context) {
  fmt.Fprint(ctx.Response, "hello world")
}


func TestSimpleRoute(t *testing.T) {
    r, err := NewRoute("^foo/$", &routableFunc{Function: viewFunc})

    if err != nil {
        t.Error("Failed to create route.")
    }
    if !r.Match("foo/") {
        t.Error("Failed to match itself")
    }

    invalid_paths := []string{
        "foo",
        "bar/",
        "bar",
        "foo/bar/",
    }
    for _, path := range invalid_paths {
        if r.Match(path) {
            t.Errorf("Matched invalid path: %s", path)
        }
    }
}
