// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
This is a simple example that demonstrates how to serve an page via FastCGI
*/

package main

import (
    "github.com/calebbrown/uweb"
)


func index() string {
    return `<html>
<head><title>Hello, World!</title></head>
<body><h1>Hello, World! (via FastCGI)</h1></body>
</html>`
}

func main() {
    uweb.Config.Debug = true
    uweb.Get("^$", index)
    if err := uweb.RunFcgi("localhost:6063"); err != nil {
        panic(err)
    }
}
