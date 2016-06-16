// Copyright 2015 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"text/template"
	"time"

	"github.com/vdobler/ht/recorder"
	"github.com/vdobler/ht/sanitize"
)

var cmdRecord = &Command{
	RunArgs:     runRecord,
	Usage:       "record [flags] <remote-target>",
	Description: "run reverse proxy to record tests",
	Flag:        flag.NewFlagSet("record", flag.ContinueOnError),
	Help: `
Record acts as a reverse proxy to <remote-target> capturing requests and
responses. It allows to filter which request/response pairs get captured.
Tests can be generated for the captured reqest/response pairs.
`,
}

func init() {
	cmdRecord.Flag.StringVar(&recorderPort, "port", ":8080", "local service port")
	cmdRecord.Flag.StringVar(&recorderLocal, "local", "localhost:8080", "local service address")
	cmdRecord.Flag.StringVar(&recorderIgnPath, "ignore.path", "",
		"ignore path matching `regexp`")
	cmdRecord.Flag.StringVar(&recorderIgnCT, "ignore.type", "",
		"ignore content types matching `regexp`")
	cmdRecord.Flag.DurationVar(&recorderDisarm, "disarm", 1*time.Second,
		"disarm recorder for `period` after last capture")
	cmdRecord.Flag.IntVar(&recorderRewrite, "rewrite", 3,
		"rewrite RespHeader=1 RespBody=2 ReqHeader=4 ReqBody=8")
	addOutputFlag(cmdRecord.Flag)
}

var (
	recorderPort    string
	recorderLocal   string
	recorderDisarm  time.Duration
	recorderIgnPath string
	recorderIgnCT   string
	recorderRewrite int
)

func runRecord(cmd *Command, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "Missing <remote-target> for record")
		fmt.Fprintf(os.Stderr, "Usage: %s\n", cmd.Usage)
		os.Exit(1)
	}

	remote, err := url.Parse(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parsee %q as an URL: %s\n", args[0], err)
		os.Exit(1)
	}

	templ = template.Must(template.New("admin").Parse(adminTemplate))
	registerAdminHandlers()

	rewrite := recorder.NewRewriter(recorderLocal, remote.Host, uint32(recorderRewrite))

	opts := recorder.Options{
		Disarm:  recorderDisarm,
		Rewrite: rewrite,
	}
	if recorderIgnPath != "" {
		opts.IgnoredPath = regexp.MustCompile(recorderIgnPath)
	}
	if recorderIgnCT != "" {
		opts.IgnoredContentType = regexp.MustCompile(recorderIgnCT)
	}

	err = recorder.StartReverseProxy(recorderPort, remote, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot launch reverse proxy: %s", err)
		os.Exit(1)
	}
}

func registerAdminHandlers() {
	http.HandleFunc("/-ADMIN-", adminHandler)
	log.Printf("Point browser to http://localhost%s/-ADMIN- to access recorder admin interface", recorderPort)
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
		suite = "suite"
	}

	dir = sanitize.Filename(dir)
	suite = sanitize.Filename(suite)

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

		w.Header().Set("Location", fmt.Sprintf("http://localhost%s/-ADMIN-", recorderPort))
		http.Error(w, "Redirect after POST", http.StatusMovedPermanently)
	}

	buf := &bytes.Buffer{}

	type Data struct {
		Dir    string
		Events []recorder.Event
	}

	if outputDir == "" {
		outputDir = time.Now().Format("2006-01-02_15h04m05s")
	}

	data := Data{
		Dir:    outputDir,
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
      <tr>
        <td style="background-color: red">Delete</td>
        <td>Name</td>
        <td>Method</td>
        <td>Content Type</td>
        <td>URL</td>
      </tr>
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
              {{$e.Request.Method}}
          </td>
          <td>
              {{with index $e.Response.HeaderMap "Content-Type"}}
                {{if gt (len .) 0}}{{index . 0}}{{end}}
              {{end}}
          </td>
          <td>
              <a href="{{$e.Request.URL}}">{{$e.Request.URL}}</a>
          </td>
      </tr>
      {{end}}
    </tbody>
  </table>

  <div style="padding: 2ex 2ex 4ex 0ex">
      <input type="submit" name="action" value="Update" /> (Delete selected and/or change names.)
  </div>

  <div>
      Save tests and suite to directory: <input type="text" name="directory" value="{{.Dir}}" />
      as suite  <input type="text" name="suite" value="suite.suite" />
      <input type="submit" name="action" value="Save" />
  </div>

</form>

</body>
`
