// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/vdobler/ht/gui"
	"github.com/vdobler/ht/ht"
	"github.com/vdobler/ht/internal/hjson"
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

To work on a test.ht which is the fifth test in suite.suite execute the
first four test in the suite storing variable and cookie state like this

    $ ht exec -only 1-4 -vardump vars -cookiedump cookies suite.suite

and open the GUI reading in this state:

    $ ht gui -Dfile vars -cookies cookies test.ht

Please note that the exported tests are not suitable for direct execution:
All durations are in nanoseconds you have to change these manually,
variables have been replaced unconditionaly during loading of the test
and have to be reintroduced.
	`,
}

func init() {
	addOutputFlag(cmdGUI.Flag)
	addVarsFlags(cmdGUI.Flag)
	addCookieFlag(cmdGUI.Flag)
	registerGUITypes()
	registerGUIImplements()
}

func runGUI(cmd *Command, tests []*suite.RawTest) {

	test := &ht.Test{}

	if len(tests) > 1 {
		log.Println("Only one test file allowed for gui.")
		os.Exit(9)
	}

	jar := loadCookies()

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
		test.Jar = jar
	}

	testValue := gui.NewValue(*test, "Test")

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/display", displayHandler(testValue))
	http.HandleFunc("/update", updateHandler(testValue))
	http.HandleFunc("/export", exportHandler(testValue))
	http.HandleFunc("/binary", binaryHandler(testValue))
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

func exportHandler(val *gui.Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")

		// Clear stuff from test whcih is not part of a Hjson test definition.
		test := val.Current.(ht.Test)
		test.Response = ht.Response{}
		test.ExValues = make(map[string]ht.Extraction)
		test.CheckResults = nil

		// Serialize to JSON as this honours json:",omitempty" and uses
		// custom marshallers for CheckList (and ExtractorMap ???)
		data, err := json.Marshal(test)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Cannot marshal to JSON: %s", err)
			return
		}

		var soup interface{}
		err = hjson.Unmarshal(data, &soup)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Cannot unmarshal to Hjson soup: %s", err)
			return
		}
		delete(soup.(map[string]interface{}), "Response")

		data, err = hjson.Marshal(soup)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "Cannot marshal to Hjson: %s", err)
			return
		}

		w.WriteHeader(200)
		w.Write(data)
	}
}

func binaryHandler(val *gui.Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		path := req.Form.Get("path")
		if path == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(500)
			w.Write([]byte("Missing path parameter"))
			return
		}

		data, err := val.BinaryData(path)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(200)
		w.Write(data)
	}
}

func updateHandler(val *gui.Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		fragment, _ := val.Update(req.Form)

		switch req.Form.Get("action") {
		case "execute":
			executeTest(val)
			extractVars(val)
			fragment = "Test.Response"
		case "runchecks":
			executeChecks(val)
			fragment = "Test.Checks"
		case "extractvars":
			extractVars(val)
			fragment = "Test.VarEx"
		case "export":
			w.Header().Set("Location", "/export")
			w.WriteHeader(303)
			return
		}

		if fragment != "" {
			fragment = "#" + fragment
		}

		w.Header().Set("Location", "/display"+fragment)
		w.WriteHeader(303)
	}
}

func executeChecks(val *gui.Value) {
	val.Last = append(val.Last, val.Current)
	test := val.Current.(ht.Test)
	prepErr := test.PrepareChecks()
	if prepErr != nil {
		// TODO: find out which check failed
		val.Messages["Test.Checks"] = []gui.Message{{
			Type: "bogus",
			Text: prepErr.Error(),
		}}
		val.Current = test
		return
	}

	test.ExecuteChecks()
	augmentMessages(&test, val)

	val.Current = test
}

func extractVars(val *gui.Value) {
	val.Last = append(val.Last, val.Current)
	test := val.Current.(ht.Test)
	test.Extract()
	val.Current = test
}

func executeTest(val *gui.Value) {
	val.Last = append(val.Last, val.Current)
	test := val.Current.(ht.Test)
	test.Run()
	if test.Response.Response != nil {
		test.Response.Response.Request = nil
		test.Response.Response.TLS = nil
	}
	augmentMessages(&test, val)
	val.Current = test
}

func augmentMessages(test *ht.Test, val *gui.Value) {
	for i, cr := range test.CheckResults {
		path := fmt.Sprintf("Test.Checks.%d", i)
		status := strings.ToLower(cr.Status.String())
		text := cr.Status.String()
		if cr.Error != nil {
			text = cr.Error.Error()
		}
		val.Messages[path] = []gui.Message{{
			Type: status,
			Text: text,
		}}
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
      </p>
      <p>
        <button class="actionbutton" name="action" value="extractvars" style="background-color: #87CEEB;"> Extract Vars </button>
      </p>
      <p>
        <button class="actionbutton" name="action" value="update"> Update Values </button>
      </p>
      <p>
        <button class="actionbutton" name="action" value="export" style="background-color: #FFE4B5;"> Export Test </button>
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
