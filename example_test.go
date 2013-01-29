// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb_test

import (
    "github.com/calebbrown/uweb"
)

// This example demonstrates a custom instance of App.
func ExampleApp() {
    app := uweb.NewApp()

    app.Get("^hello/(.*)", func (name string) string {
        return "Hello, " + name
    })

    app.Post("^submit/$", func (ctx *uweb.Context) {

    })

    app.Run("localhost:6060")
}

func ExampleMount() {
    app := uweb.NewApp()

    app.Get("^bar/(.*)", func (name string) string {
        return "Hello, " + name
    })

    uweb.Mount("^foo/", app)

    uweb.Run("localhost:6060")
}

