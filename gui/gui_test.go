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
	Options     Options
	Execution   Execution
	Pointer     *int
	NilPointer  *int
	Fancy       map[string][]string
	Writer      Writer
	Writers     []Writer
}

type Options struct {
	Simple   string
	Advanced time.Duration
	Complex  struct {
		Foo string
		Bar string
	}
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
			"Name":        Fieldinfo{Doc: "Must be unique."},
			"Description": Fieldinfo{Multiline: true},
		},
	})

	RegisterType(Execution{}, Typeinfo{
		Doc: "Controles test execution",
		Field: map[string]Fieldinfo{
			"Method": Fieldinfo{
				Doc:  "The HTTP method",
				Only: []string{"GET", "POST", "HEAD"},
			},
			"Tries": Fieldinfo{
				Doc: "Number of retries.",
			},
			"Wait": Fieldinfo{
				Doc: "Sleep duration between retries.",
			},
			"Hash": Fieldinfo{
				Doc:      "Hash in hex",
				Validate: regexp.MustCompile("^[[:xdigit:]]{4,8}$"),
			},
		},
	})

	RegisterType(Options{}, Typeinfo{
		Doc: "Options captures basic settings\nfor this piece of code.",
		Field: map[string]Fieldinfo{
			"Simple":   Fieldinfo{Doc: "Simple is the common name."},
			"Advanced": Fieldinfo{Doc: "Advanced contains admin options."},
			"Complex":  Fieldinfo{Doc: "Complex allows fancy customisations."},
		},
	})

	RegisterType(AbcWriter{}, Typeinfo{
		Doc: "AbcWriter is for latin scripts",
		Field: map[string]Fieldinfo{
			"Name": Fieldinfo{Doc: "How to address this Writer."},
		},
	})

	RegisterType(XyzWriter{}, Typeinfo{
		Doc: "XyZWriter is a useless\ndummy type for testing",
		Field: map[string]Fieldinfo{
			"Count": Fieldinfo{Doc: "Ignored."},
		},
	})

	RegisterType(PtrWriter{}, Typeinfo{
		Doc: "PtrWriter: Only *PtrWriter are Writers",
		Field: map[string]Fieldinfo{
			"Full":     Fieldinfo{Doc: "Full check"},
			"Fraction": Fieldinfo{Doc: "0 <= Fraction <=1"},
		},
	})

}

var test Test
var testgui = flag.Bool("gui", false, "actually serve a GUI under :8888")
var globalRenderer Value

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
	test.Primes = []int{2, 3, 5}
	test.Execution.Tries = 3
	test.Execution.unexported = -99
	test.Options.Advanced = 150 * time.Millisecond
	test.Execution.Env = map[string]string{
		"Hello": "World",
		"ABC":   "XYZ",
	}
	var x int = 17
	test.Pointer = &x
	test.Fancy = map[string][]string{
		"Hund":  []string{"doof", "dreckig"},
		"katze": []string{"schlau"},
	}
	test.Writer = AbcWriter{"Heinz"}
	test.Writers = []Writer{XyzWriter{8}, AbcWriter{"Anna"}, &PtrWriter{}}

	value := NewValue(test, "Test")

	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/display", displayHandler(value))
	http.HandleFunc("/update", updateHandler(value))
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
<p>&nbsp;</p><input type="submit" />
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
