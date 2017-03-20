// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"io"
	"strings"
	"text/template"
)

// ----------------------------------------------------------------------------
// Status

// Status describes the status of a Test or a Check.
type Status int

// Possible status of Checks, Tests and Suites.
const (
	NotRun  Status = iota // Not jet executed
	Skipped               // Omitted deliberately
	Pass                  // That's what we want
	Fail                  // One ore more checks failed
	Error                 // Request or body reading failed (not for checks).
	Bogus                 // Bogus test or check (malformd URL, bad regexp, etc.)
)

func (s Status) String() string {
	return []string{"NotRun", "Skipped", "Pass", "Fail", "Error", "Bogus"}[int(s)]
}

//                 01234567        01234567        01234567
//                         01234567        01234567        01234567
const statusnames = "notrun  skipped pass    fail    error   bogus"

// StatusFromString parse s into a Status. If s is not a valid Status
// (i.e. one of NotRun, ..., Bogus) then -1 is returned.
func StatusFromString(s string) Status {
	s = strings.TrimSpace(strings.ToLower(s))
	i := strings.Index(statusnames, s)
	if i < 0 {
		return Status(-1)
	}
	return Status(i / 8)
}

// MarshalText implements encoding.TextMarshaler.
func (s Status) MarshalText() ([]byte, error) {
	if s < 0 || s > Bogus {
		return []byte(""), fmt.Errorf("no such status %d", s)
	}
	return []byte(s.String()), nil
}

// ----------------------------------------------------------------------------
// Templates to output

var DefaultCheckTemplate = `{{define "CHECK"}}{{printf "%-7s %-15s %s" .Status .Name .JSON}}` +
	`{{if eq .Status 3 5}}{{range .Error}}
                {{.Error}}{{end}}{{end}}{{end}}`

var DefaultTestTemplate = `{{define "TEST"}}{{ToUpper .Status.String}}: {{.Name}}{{if gt .Tries 1}}
  {{printf "(after %d tries)" .Tries}}{{end}}
  Started: {{.Started}}   Duration: {{.FullDuration}}   Request: {{.Duration}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  Error: {{.Error}}{{end}}
{{if eq .Status 2 3 4 5}}  {{if .CheckResults}}Checks:
{{range $i, $c := .CheckResults}}{{printf "    %2d. " $i}}{{template "CHECK" .}}
{{end}}{{end}}{{end}}{{if .Variables}}  Variables:
{{range $k, $v := .Variables}}{{printf "    %s == %q\n" $k $v}}{{end}}{{end}}{{if .ExValues}}  Extracted:
{{range $k, $v := .ExValues}}{{if $v.Error}}{{printf "    %s : %s\n" $k $v.Error}}{{else}}{{printf "    %s == %q\n" $k $v.Value}}{{end}}{{end}}{{end}}{{end}}`

var ShortTestTemplate = `{{define "SHORTTEST"}}{{.Status.String}}: {{.Name}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  {{.Error}}{{end}}{{if gt .Status 2}}{{if .CheckResults}}{{range .CheckResults}}{{if eq .Status 3 5}}
    {{printf "%-7s %-15s %s" .Status .Name .JSON}}{{range .Error}}
      {{.Error}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .ExValues}}{{range $k, $v := .ExValues}}{{if $v.Error}}
  {{printf "Extraction of %s : %s\n" $k $v.Error}}{{end}}{{end}}{{end}}
{{end}}`

var (
	ShortTestTmpl *template.Template
	TestTmpl      *template.Template
)

func init() {
	fm := make(template.FuncMap)
	fm["Underline"] = Underline
	fm["Box"] = Box
	fm["ToUpper"] = strings.ToUpper

	ShortTestTmpl = template.New("SHORTTEST")
	ShortTestTmpl.Funcs(fm)
	ShortTestTmpl = template.Must(ShortTestTmpl.Parse(ShortTestTemplate))

	TestTmpl = template.New("TEST")
	TestTmpl.Funcs(fm)
	TestTmpl = template.Must(TestTmpl.Parse(DefaultTestTemplate))
	TestTmpl = template.Must(TestTmpl.Parse(DefaultCheckTemplate))
}

// PrintReport of t to w.
func (t *Test) PrintReport(w io.Writer) error {
	return TestTmpl.Execute(w, t)
}

// PrintShortReport of t to w.
func (t *Test) PrintShortReport(w io.Writer) error {
	return ShortTestTmpl.Execute(w, t)
}
