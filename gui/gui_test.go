// Copyright 2017 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gui

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"testing"
	"time"
)

// ----------------------------------------------------------------------------
// An interface and two implementations.

type Writer interface {
	Write()
}
type AbcWriter struct{ Name string }

func (AbcWriter) Write() {}

type XyzWriter struct{ Count int }

func (XyzWriter) Write() {}

type PtrWriter struct {
	Full     bool
	Fraction float64
}

func (*PtrWriter) Write() {}

// ----------------------------------------------------------------------------
// Test is a demo structure

type Test struct {
	Name        string
	Description string
	Primes      []int
	Output      string
	Chan        chan string
	Func        func(int) bool
	Options     Options
	Execution   Execution
	Result      Result
	Pointer     *int
	NilPointer  *int
	Fancy       map[string][]string
	Writer      Writer
	Writers     []Writer
}

type Result struct {
	Okay     bool
	Count    int
	State    string
	Frac     float64
	Details  []string
	Options  *Options
	Vars     map[string]string
	Duration time.Duration
	Time     time.Time
}

type Options struct {
	Simple   string
	Advanced time.Duration
	Complex  struct {
		Foo string
		Bar string
	}
	Started time.Time
	Data    []byte
	Ignore  string
}

type Execution struct {
	Method     string
	Tries      int
	Wait       time.Duration
	Hash       string
	Env        map[string]string
	unexported int
}

func registerTestTypes() {
	RegisterType(Test{}, Typeinfo{
		Doc: "Test to perform.",
		Field: map[string]Fieldinfo{
			"Name":        {Doc: "Must be unique."},
			"Description": {Multiline: true},
			"Output":      {Const: true},
			"Result":      {Const: true},
		},
	})

	RegisterType(Execution{}, Typeinfo{
		Doc: "Controles test execution",
		Field: map[string]Fieldinfo{
			"Method": {
				Doc:  "The HTTP method",
				Only: []string{"GET", "POST", "HEAD"},
			},
			"Tries": {
				Doc: "Number of retries.",
			},
			"Wait": {
				Doc: "Sleep duration between retries.",
			},
			"Hash": {
				Doc:      "Hash in hex",
				Validate: regexp.MustCompile("^[[:xdigit:]]{4,8}$"),
			},
		},
	})

	RegisterType(Options{}, Typeinfo{
		Doc: "Options captures basic settings\nfor this piece of code.",
		Field: map[string]Fieldinfo{
			"Simple":   {Doc: "Simple is the common name."},
			"Advanced": {Doc: "Advanced contains admin options."},
			"Complex":  {Doc: "Complex allows fancy customisations."},
			"Ignore": {
				Doc: "Ignore some weather conditions:\n" +
					"Space seperated list of 'rain', 'darkness', 'snow' and 'hail'",
				Any: []string{"rain", "rainfall", "snow", "hail"},
			},
		},
	})

	RegisterType(Result{}, Typeinfo{
		Doc: "Result of the Test",
	})

	RegisterType(AbcWriter{}, Typeinfo{
		Doc: "AbcWriter is for latin scripts",
		Field: map[string]Fieldinfo{
			"Name": {Doc: "How to address this Writer."},
		},
	})

	RegisterType(XyzWriter{}, Typeinfo{
		Doc: "XyZWriter is a useless\ndummy type for testing",
		Field: map[string]Fieldinfo{
			"Count": {Doc: "Ignored."},
		},
	})

	RegisterType(PtrWriter{}, Typeinfo{
		Doc: "PtrWriter: Only *PtrWriter are Writers",
		Field: map[string]Fieldinfo{
			"Full":     {Doc: "Full check"},
			"Fraction": {Doc: "0 <= Fraction <=1"},
		},
	})

}

var test Test
var testgui = flag.Bool("gui", false, "actually serve a GUI under :8888")

func TestGUI(t *testing.T) {
	if !*testgui {
		t.Skip("Can be executed via cmdline flag -gui")
	}

	// Register types and implementations.
	Typedata = make(map[reflect.Type]Typeinfo)
	registerTestTypes()

	Implements = make(map[reflect.Type][]reflect.Type)
	RegisterImplementation((*Writer)(nil), AbcWriter{})
	RegisterImplementation((*Writer)(nil), XyzWriter{})
	RegisterImplementation((*Writer)(nil), &PtrWriter{})

	// Fill initial values of a Test.
	test = Test{}
	test.Name = "Hello World"
	test.Output = "Outcome of the Test is 'good'.\nNext run maybe too..."
	test.Primes = []int{2, 3, 5}
	test.Execution.Tries = 3
	test.Execution.unexported = -99
	test.Options.Advanced = 150 * time.Millisecond
	test.Options.Started = time.Now()
	test.Options.Data = []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00,
		0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x10,
		0x00, 0x00, 0x00, 0x10, 0x08, 0x02, 0x00, 0x00, 0x00, 0x90,
		0x91, 0x68, 0x36, 0x00, 0x00, 0x00, 0x2E, 0x49, 0x44, 0x41,
		0x54, 0x28, 0xCF, 0x63, 0x60, 0x18, 0x74, 0x80, 0x71, 0x9A,
		0x97, 0x17, 0x49, 0x1A, 0x98, 0x48, 0xB5, 0x81, 0x64, 0x0D,
		0x2C, 0x99, 0x7C, 0x7C, 0x83, 0xCC, 0x49, 0x8C, 0x26, 0x31,
		0xEF, 0xB1, 0x4A, 0x48, 0xD8, 0x78, 0x0C, 0x90, 0x93, 0x86,
		0x83, 0x06, 0x00, 0xC6, 0x09, 0x03, 0xE5, 0x67, 0xDA, 0x39,
		0xCE, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82}
	test.Options.Ignore = "rainfall snow"
	test.Execution.Env = map[string]string{
		"Hello": "World",
		"ABC":   "XYZ",
	}
	var x int = 17
	test.Pointer = &x
	test.Fancy = map[string][]string{
		"Hund":  {"doof", "dreckig"},
		"katze": {"schlau"},
	}
	test.Writer = AbcWriter{"Heinz"}
	test.Writers = []Writer{XyzWriter{8}, AbcWriter{"Anna"}, &PtrWriter{}}
	test.Result.Okay = true
	test.Result.Count = 137
	test.Result.State = "Passed"
	test.Result.Frac = 0.75
	test.Result.Details = []string{"Executed", "Worked\nas intended", "<<super>>"}
	test.Result.Options = nil
	test.Result.Vars = map[string]string{"DE": "Deutsch", "FR": "Français"}
	test.Result.Duration = 137 * time.Millisecond
	test.Result.Time = time.Now()

	value := NewValue(test, "Test")

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/display", displayHandler(value))
	http.HandleFunc("/update", updateHandler(value))
	http.HandleFunc("/binary", binaryHandler(value))
	log.Fatal(http.ListenAndServe(":8888", nil))
}

func displayHandler(val *Value) func(w http.ResponseWriter, req *http.Request) {
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

func updateHandler(val *Value) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		fragment, errlist := val.Update(req.Form)

		for _, e := range errlist {
			fmt.Println(e)
		}

		if fragment != "" {
			fragment = "#" + fragment
		}
		w.Header().Set("Location", "/display"+fragment)
		w.WriteHeader(303)
	}
}

func binaryHandler(val *Value) func(w http.ResponseWriter, req *http.Request) {
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
		w.Header().Set("Content-Disposition", "inline")
		w.WriteHeader(200)
		w.Write(data)
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
	buf.WriteString(CSS)
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
</body>
</html>
`)

}

func faviconHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Write(Favicon)
}
