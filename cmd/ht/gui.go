// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/vdobler/ht/gui"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/scope"
	"github.com/vdobler/ht/suite"
)

var cmdGUI = &Command{
	RunTests:    runGUI,
	Usage:       "gui [<test>]",
	Description: "Edit and debug a test in a GUI",
	Flag:        flag.NewFlagSet("gui", flag.ContinueOnError),
	Help: `
Gui provides a HTML GUI to create, edit and modifiy test.
	`,
}

func init() {
	addOutputFlag(cmdGUI.Flag)
	addVarsFlags(cmdGUI.Flag)

	registerGUITypes()
	registerGUIImplements()
}

func runGUI(cmd *Command, tests []*suite.RawTest) {

	test := &ht.Test{}

	if len(tests) > 1 {
		log.Println("Only one suite allowed for gui.")
		os.Exit(9)
	}

	if len(tests) == 1 {
		rt := tests[0]
		testScope := scope.New(scope.Variables(variablesFlag), rt.Variables, false)
		testScope["TEST_DIR"] = rt.File.Dirname()
		testScope["TEST_NAME"] = rt.File.Basename()
		var err error
		test, err = rt.ToTest(testScope)
		if err != nil {
			log.Println(err)
			os.Exit(9)
		}
		test.SetMetadata("Filename", rt.File.Name)
	}

	testValue := gui.NewValue(test, "Test")

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/display", displayHandler(testValue))
	http.HandleFunc("/update", updateHandler(testValue))
	fmt.Println("Open GUI on http://localhost:8888/display")
	log.Fatal(http.ListenAndServe(":8888", nil))

	os.Exit(0)
}

func displayHandler(val *gui.Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		buf := &bytes.Buffer{}
		writePreamble(buf, "Test")
		data, err := val.Render()
		buf.Write(data)
		writeEpilogue(buf)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		w.Write(buf.Bytes())

		if err != nil {
			log.Fatal(err)
		}
	}
}

func updateHandler(val *gui.Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		_, errlist := val.Update(req.Form)

		if len(errlist) == 0 {
			w.Header().Set("Location", "/display")
			w.WriteHeader(303)
			return
		}

		fmt.Println(errlist)

		buf := &bytes.Buffer{}
		writePreamble(buf, "Bad input")
		data, _ := val.Render()
		buf.Write(data)
		writeEpilogue(buf)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(400)
		w.Write(buf.Bytes())
	}
}

func writePreamble(buf *bytes.Buffer, title string) {
	buf.WriteString(`<!doctype html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Check Builder</title>
    <style>
 `)
	buf.WriteString(gui.CSS)
	buf.WriteString(`
.valueform {
  margin-right: 240px;
}
    </style>
</head>
<body>
  <h1>` + title + `</h1>
  <div class="valueform">
  <form action="/update" method="post">
`)
}

func writeEpilogue(buf *bytes.Buffer) {
	buf.WriteString(`
    <div style="position: fixed; top:2%; right:2%;">
      </p>
        <button class="actionbutton" name="action" value="execute" style="background-color: #DDA0DD;"> Execute Test </button>
      </p>
      <p>
        <button class="actionbutton" name="action" value="runchecks" style="background-color: #FF8C00;"> Try Checks </button>
      <p>
        <button class="actionbutton" name="action" value="update"> Update Values </button>
      </p>
    </div>

  </form>
  </div>
</body>
</html>
`)

}

func faviconHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Write(gui.Favicon)
}
