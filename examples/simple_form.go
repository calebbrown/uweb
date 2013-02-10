// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
This is a simple example that demonstrates how to show a form and handle
submissions.
*/

package main

import (
	"bytes"
	"encoding/json"
	"github.com/calebbrown/uweb"
	"html/template"
	"os"
)

// Where to store the message data
const MessageFile = "messages.json"

// The template to use to render the index page
const IndexTemplate = `<html>
<body>
<form action="/save/" method="POST">
<input type="text" name="message" autofocus>
<button>New Message</button>
</form>
{{range $index, $message := .Messages }}<div>{{ $message }}</div>
{{end}}
</body>
</html>`

func loadMessages() []string {
	// Open the file for reading
	// If it doesn't open, just assume there are no messages
	file, err := os.Open(MessageFile)
	if err != nil {
		return make([]string, 0)
	}
	defer file.Close()

	// Read in the messages from the json file
	var res []string
	dec := json.NewDecoder(file)
	dec.Decode(&res)

	return res
}

func saveMessage(message string) {
	// Load the original messages into memory
	messages := loadMessages()
	messages = append(messages, message)

	// Open the file for writing
	file, err := os.Create(MessageFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Write the messages as json into the file
	enc := json.NewEncoder(file)
	enc.Encode(messages)
}

func index() *bytes.Buffer {
	t, err := template.New("index").Parse(IndexTemplate)
	if err != nil {
		panic(err)
	}

	messages := loadMessages()

	// reverse the messages so they appear in descending order
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	// render the template
	var b bytes.Buffer
	err = t.Execute(&b, struct{ Messages []string }{Messages: messages})
	if err != nil {
		panic(err)
	}

	return &b
}

func save(ctx *uweb.Context) {
	message := ctx.Request.FormValue("message")
	saveMessage(message)
	uweb.Redirect("/")
}

func main() {
	uweb.Get("^$", index)
	uweb.Post("^save/$", save)
	uweb.Run("localhost:6062")
}
