// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/mgutz/ansi"
	"github.com/vdobler/ht/check"
)

// Status describes the status of a Test or a Check.
type Status int

func (s Status) String() string {
	return []string{"NotRun", "Skipped", "Pass", "Fail", "Error", "Bogus"}[int(s)]
}

func (s Status) MarshalText() ([]byte, error) {
	if s < 0 || s > Bogus {
		return []byte(""), fmt.Errorf("no such status %d", s)
	}
	return []byte(s.String()), nil
}

const (
	NotRun  Status = iota // Not jet executed
	Skipped               // Omitted deliberately
	Pass                  // That's what we want
	Fail                  // One ore more checks failed
	Error                 // Request or body reading failed (not for checks).
	Bogus                 // Bogus test or check (malformd URL, bad regexp, etc.)
)

// Result is the outcome of a Test or Check.
type Result struct {
	Status       Status
	Name         string        `json:",omitempty"`
	Error        error         `json:",omitempty"`
	Duration     time.Duration `json:",omitempty"`
	FullDuration time.Duration `json:",omitempty"`
	Details      string        `json:",omitempty"`
	Elements     []Result
}

var txtTmpl *template.Template

func init() {
	txtTmpl = template.New("TextReport")
	fm := make(template.FuncMap)
	fm["Underline"] = Underline
	fm["Box"] = Box
	txtTmpl.Funcs(fm)
	var err error
	txtTmpl, err = txtTmpl.Parse(`
{{Underline (printf "TEST: %s" .Name) "=" ""}}
{{if .Details}}{{.Details}}
{{end}}
{{Underline (printf "Status: %s" .Status) "~" ""}}
{{if eq .Status 2 3 4 5}}
Test: {{.FullDuration}}   Request: {{.Duration}}  
{{if .Error}}Error: {{.Error}}{{end}}
{{if .Elements}}Checks:
{{range $i, $c := .Elements}}{{printf "  %2d. %-7s %-15s %-15s" $i $c.Status $c.Name $c.Details}}
{{if eq $c.Status 3 5}}{{printf "                              %s\n" $c.Error.Error}}{{end}}{{end}}
{{else}}No checks{{end}}{{end}}
`)
	if err != nil {
		panic(err.Error())
	}
}

// underline title with c
func underline(title string, c string) string {
	return title + "\n" + strings.Repeat(c, len(title))[:len(title)]
}

// box around title
func box(title string) string {
	n := len(title)
	top := "+" + strings.Repeat("-", n+6) + "+"
	return fmt.Sprintf("%s\n|   %s   |\n%s", top, title, top)
}

func (t *Test) PrintReport(w io.Writer, result Result) error {
	// Fill descriptive data in various results for display.
	// TODO: maybe extract to own method or do during prepare?
	result.Name = t.Name
	result.Details = t.Description
	if len(result.Elements) == 0 {
		println("Ooops ", t.Name)
		result.Elements = make([]Result, len(t.Checks))
	}
	for i := range t.Checks {
		result.Elements[i].Name = check.NameOf(t.Checks[i])
		j, err := json.Marshal(t.Checks[i])
		if err != nil {
			panic(err.Error())
		}

		result.Elements[i].Details = string(j)
	}

	return txtTmpl.Execute(os.Stdout, result)
}

func printReport() {
	pass := ansi.ColorFunc("green")
	error := ansi.ColorFunc("red+b")
	fail := ansi.ColorFunc("red")
	fmt.Printf("Test 17 'WhiteFrog': %s\n Err = %s\n Fail=%s",
		pass("PASS"), error("Foo"), fail("Blubs"))
}
