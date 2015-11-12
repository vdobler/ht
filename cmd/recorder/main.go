// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// recorder is a reverse proxy to record requests and output tests.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/vdobler/ht/recorder"
)

var (
	port      = flag.String("port", ":8080", "local service address")
	directory = flag.String("dir", "./recorded", "save tests to directory `d`")
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Missing target\n")
		os.Exit(9)
	}

	remote, err := url.Parse(args[0])
	if err != nil {
		panic(err)
	}

	templ = template.Must(template.New("admin").Parse(adminTemplate))
	registerAdminHandlers()

	err = recorder.StartReverseProxy(*port, remote)
	if err != nil {
		panic(err)
	}
}

func registerAdminHandlers() {
	http.HandleFunc("/-ADMIN-", adminHandler)
	log.Printf("Point browser to http://localhost%s/-ADMIN- to access recorder admin interface", *port)
}

func updateEvents(form url.Values) error {
	del := map[int]bool{}
	for _, v := range form["event"] {
		i, err := strconv.Atoi(v)
		if err != nil {
			return err
		}
		del[i] = true
	}
	log.Printf("Del == %#v", del)
	ne := []recorder.Event{}
	for i, e := range recorder.Events {
		if del[i] {
			continue
		}
		if name := form.Get(fmt.Sprintf("name%d", i)); name != "" {
			e.Name = name
		}
		ne = append(ne, e)
	}
	recorder.Events = ne
	return nil
}

func saveEvents(form url.Values) error {
	ets := []recorder.Event{}
	for i, e := range recorder.Events {
		if name := form.Get(fmt.Sprintf("name%d", i)); name != "" {
			e.Name = name
		}
		ets = append(ets, e)
	}
	dir := form.Get("directory")
	if dir == "" {
		dir = "."
	}
	suite := form.Get("suite")
	if suite == "" {
		suite = "Suite"
	}
	err := recorder.DumpEvents(ets, dir, suite)
	if err != nil {
		return err
	}
	log.Printf("Saved %d tests to directory %s", len(ets), dir)

	recorder.Events = recorder.Events[:0]
	return nil
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	if r.Method == "POST" {
		switch r.FormValue("action") {
		case "Update":
			err = updateEvents(r.Form)
		case "Save":
			err = saveEvents(r.Form)
		default:
			err = fmt.Errorf("Unknown action %q", r.FormValue("action"))
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	buf := &bytes.Buffer{}

	type Data struct {
		Dir    string
		Events []recorder.Event
	}

	data := Data{
		Dir:    *directory,
		Events: recorder.Events,
	}

	err = templ.Execute(buf, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(buf.Bytes())
}

var templ *template.Template

const adminTemplate = `<!DOCTYPE html>
<head>
   <meta http-equiv="content-type" content="text/html; charset=utf-8">
   <title>Test Recoder - Admin</title>
</head>
<body>
<h1>Test Recorder - Admin</h1>

<form action="/-ADMIN-" method="post">
  <table style="border-spacing: 5px">
    <thead>
      <tr><td style="background-color: red">Delete</td><td>Name</td><td>URL</td></tr>
    </thead>
    <tbody>
      {{range $i, $e := .Events}}
      <tr style="padding-bottom: 1ex">
          <td>
              <input type="checkbox" name="event" value="{{$i}}" />
          </td>
          <td>
              <input type="text" name="name{{$i}}" value="{{$e.Name}}" />
          </td>
          <td>
              {{$e.Request.URL}}
          </td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <div style="padding: 2ex 2ex 4ex 0ex">
      <input type="submit" name="action" value="Update" />
  </div>

  <div>
      Save to: <input type="text" name="directory" value="{{.Dir}}" />
      as suite  <input type="text" name="suite" value="" />
      <input type="submit" name="action" value=" Save " />
  </div>

</form>

</body>
`
