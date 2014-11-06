// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	"io"
	"os"
	"text/template"
	"time"

	"github.com/vdobler/ht/response"
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

// SuiteResult captuires the outcome of running a whole suite.
type SuiteResult struct {
	Name         string
	Description  string
	Status       Status
	Error        error
	Started      time.Time // Start time
	FullDuration time.Duration
	TestResults  []TestResult
}

// CombineTests returns the combined status of the Tests in sr.
func (sr SuiteResult) CombineTests() Status {
	status := NotRun
	for _, r := range sr.TestResults {
		if r.Status > status {
			status = r.Status
		}
	}
	return status
}

func (sr SuiteResult) Stats() (notRun int, skipped int, passed int, failed int, errored int, bogus int) {
	for _, tr := range sr.TestResults {
		switch tr.Status {
		case NotRun:
			notRun++
		case Skipped:
			skipped++
		case Pass:
			passed++
		case Fail:
			failed++
		case Error:
			errored++
		case Bogus:
			bogus++
		default:
			panic(fmt.Sprintf("No such Status %d in suite %q test %q",
				tr.Status, sr.Name, tr.Name))
		}
	}
	return
}

// TestResults captures the outcome of a single test run.
type TestResult struct {
	Name         string             // Name of the test.
	Description  string             // Copy of the description of the test
	Status       Status             // The outcume of the test.
	Started      time.Time          // Start time
	Error        error              // Error of bogus and errored tests.
	Response     *response.Response // The received response.
	Duration     time.Duration      // A copy of Response.Duration
	FullDuration time.Duration      // Total time of test execution, including tries.
	Tries        int                // Number of tries executed.
	CheckResults []CheckResult      // The individual checks.
}

// CombineChecks returns the combined status of the Checks in tr.
func (tr TestResult) CombineChecks() Status {
	status := NotRun
	for _, r := range tr.CheckResults {
		if r.Status > status {
			status = r.Status
		}
	}
	return status
}

// CheckResult captures the outcom of a single check inside a test.
type CheckResult struct {
	Name     string        // Name of the check as registered.
	JSON     string        // JSON serialization of check.
	Status   Status        // Outcome of check. All status but Error
	Duration time.Duration // How long the check took.
	Error    error         // For a Status of Bogus or Fail.
}

var defaultCheckTmpl = `{{define "CHECK"}}{{printf "%-7s %-15s %s" .Status .Name .JSON}}` +
	`{{if eq .Status 3 5}} {{.Error.Error}}{{end}}{{end}}`

var defaultTestTmpl = `{{define "TEST"}}{{ToUpper .Status.String}}: {{.Name}}{{if gt .Tries 1}}
  {{printf "(after %d tries)" .Tries}}{{end}}
  Started: {{.Started}}   Duration: {{.FullDuration}}   Request: {{.Duration}}{{if .Error}}
  Error: {{.Error}}{{end}}{{if eq .Status 2 3 4 5}}
  {{if .CheckResults}}Checks:
{{range $i, $c := .CheckResults}}{{printf "    %2d. " $i}}{{template "CHECK" .}}
{{end}}{{end}}{{end}}{{end}}`

var defaultSuiteTmpl = `{{Box (printf "%s: %s" (ToUpper .Status.String) .Name) ""}}{{if .Error}}
Error: {{.Error}}{{end}}
Started: {{.Started}}   Duration: {{.FullDuration}}
Individual tests:
{{range .TestResults}}{{template "TEST" .}}{{end}}
`

var (
	TestTmpl  *template.Template
	SuiteTmpl *template.Template
)

func init() {
	fm := make(template.FuncMap)
	fm["Underline"] = Underline
	fm["Box"] = Box
	fm["ToUpper"] = ToUpper

	TestTmpl = template.New("TEST")
	TestTmpl.Funcs(fm)
	TestTmpl = template.Must(TestTmpl.Parse(defaultTestTmpl))
	TestTmpl = template.Must(TestTmpl.Parse(defaultCheckTmpl))

	SuiteTmpl = template.New("SUITE")
	SuiteTmpl.Funcs(fm)
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultSuiteTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultTestTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultCheckTmpl))

}

func (r TestResult) PrintReport(w io.Writer) error {
	return TestTmpl.Execute(os.Stdout, r)
}

func (r SuiteResult) PrintReport(w io.Writer) error {
	return SuiteTmpl.Execute(os.Stdout, r)
}
