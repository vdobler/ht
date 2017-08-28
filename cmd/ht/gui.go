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

		fmt.Printf("updatehandler: err=%v <%T>\n", errlist, errlist)

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
    </style>
</head>
<body>
  <h1>` + title + `</h1>
  <form action="/update" method="post">
`)
}

func writeEpilogue(buf *bytes.Buffer) {
	buf.WriteString(`
<p>&nbsp;</p><input type="submit" />
  </form>
</body>
</html>
`)

}

func faviconHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Write(gui.Favicon)
}

func registerGUITypes() {
	gui.RegisterType(ht.Test{}, gui.Typeinfo{
		Doc: "The Test",
		Field: map[string]gui.Fieldinfo{
			"Name":         gui.Fieldinfo{Doc: "Short name of the test"},
			"Description":  gui.Fieldinfo{Multiline: true},
			"Request":      gui.Fieldinfo{Doc: "The HTTP Request"},
			"Checks":       gui.Fieldinfo{Doc: "The checks to perform"},
			"Execution":    gui.Fieldinfo{Doc: "Control the test execution"},
			"Jar":          gui.Fieldinfo{Omit: true},
			"Log":          gui.Fieldinfo{Omit: true},
			"Variables":    gui.Fieldinfo{Doc: "Variables contains name/value-pairs used for variable substitution\nin files read in, e.g. for Request.Body = \"@vfile:/path/to/file\"."},
			"Response":     gui.Fieldinfo{Doc: "The received response", Const: true},
			"Status":       gui.Fieldinfo{Doc: "Test status. 0=NotRun 1=Skipped 2=Pass 3=Fail 4=Error 5=Bogus", Const: true},
			"Error":        gui.Fieldinfo{Doc: "Error", Const: true},
			"Duration":     gui.Fieldinfo{Doc: "Duration of last test execution", Const: true},
			"FullDuration": gui.Fieldinfo{Doc: "Overal duration including retries", Const: true},
			"Tries":        gui.Fieldinfo{Doc: "Number of tries executed", Const: true},
			"CheckResults": gui.Fieldinfo{Doc: "The outcome of the checks", Const: true},
			"VarEx":        gui.Fieldinfo{Doc: "Extract variables"},
			"ExValues":     gui.Fieldinfo{Doc: "Extracted values", Const: true},
		},
	})

	gui.RegisterType(ht.Execution{}, gui.Typeinfo{
		Doc: "Parameters controlling the test execution.",
		Field: map[string]gui.Fieldinfo{
			"Tries": gui.Fieldinfo{Doc: `Tries is the maximum number of tries made for this test.
Both 0 and 1 mean: "Just one try. No redo."
Negative values indicate that the test should be skipped
altogether.`},
			"Wait":       gui.Fieldinfo{Doc: `Wait time between retries.`},
			"PreSleep":   gui.Fieldinfo{Doc: `Sleep time before request`},
			"InterSleep": gui.Fieldinfo{Doc: `Sleep time between request and checks`},
			"PostSleep":  gui.Fieldinfo{Doc: `Sleep time after checks`},
			"Verbosity":  gui.Fieldinfo{Doc: `Verbosity level in logging.`},
		},
	})

	gui.RegisterType(ht.Request{}, gui.Typeinfo{
		Doc: "The HTTP request.",
		Field: map[string]gui.Fieldinfo{
			"Method": gui.Fieldinfo{Doc: `Method is the HTTP method to use.
A empty method is equivalent to "GET"`,
				Only: []string{"GET", "POST", "HEAD", "PUT", "DELETE", "PATCH"}},
			"URL": gui.Fieldinfo{Doc: `the URL of the request`},
			"Params": gui.Fieldinfo{Doc: `Params contains the parameters and their values to send in
the request.

If the parameters are sent as multipart it is possible to include
files by special formated values.
The following formats are recognized:
   @file:/path/to/thefile
        read in /path/to/thefile and use its content as the
        parameter value. The path may be relative.
   @vfile:/path/to/thefile
        read in /path/to/thefile and perform variable substitution
        in its content to yield the parameter value.
   @file:@name-of-file:direct-data
   @vfile:@name-of-file:direct-data
        use direct-data as the parameter value and name-of-file
        as the filename. (There is no difference between the
        @file and @vfile variants; variable substitution has
        been performed already and is not done twice on direct-data.`},
			"ParamsAs": gui.Fieldinfo{Doc: `determines how the parameters in the Param field are sent:
  "URL" or "": append properly encoded to URL
  "body"     : send as application/x-www-form-urlencoded in body.
  "multipart": send as multipart/form-data in body.
The two values "body" and "multipart" must not be used
on a GET or HEAD request.`,
				Only: []string{"URL", "body", "multipart"}},
			"Header": gui.Fieldinfo{Doc: `Header contains the specific http headers to be sent in this request.
User-Agent and Accept headers are set automaticaly to the global
default values if not set explicitly.`},
			"Cookies": gui.Fieldinfo{Doc: `the cookies to send in the request`},
			"Body": gui.Fieldinfo{Doc: `the full body to send in the request. Body must be
empty if Params are sent as multipart or form-urlencoded.
The @file: and @vfile: prefixes are recognised and work like described
in Params`, Multiline: true},
			"FollowRedirects": gui.Fieldinfo{Doc: `Check to follow redirect automatically`},
			"BasicAuthUser":   gui.Fieldinfo{Doc: `Username to sen in Basic Auth header`},
			"BasicAuthPass":   gui.Fieldinfo{Doc: `Password to sen in basic Auth header`},
			"Chunked": gui.Fieldinfo{Doc: `turns of setting of the Content-Length header resulting
in chunked transfer encoding of POST bodies`},
			"Timeout":    gui.Fieldinfo{Doc: `of the request, 0 means the defaults to 10s`},
			"Request":    gui.Fieldinfo{Doc: `The underlying Go http.Request`, Const: true},
			"SentBody":   gui.Fieldinfo{Doc: `The actual sent body data`, Multiline: true, Const: true},
			"SentParams": gui.Fieldinfo{Doc: `The actual sent parameters`, Const: true},
		},
	})
}
