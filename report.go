// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"text/template"
	"time"

	"github.com/vdobler/ht/response"
)

// ----------------------------------------------------------------------------
// Status

// Status describes the status of a Test or a Check.
type Status int

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

func (s Status) MarshalText() ([]byte, error) {
	if s < 0 || s > Bogus {
		return []byte(""), fmt.Errorf("no such status %d", s)
	}
	return []byte(s.String()), nil
}

// ----------------------------------------------------------------------------
// SuiteResult

// SuiteResult captures the outcome of running a whole suite.
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

// Stats counts the test results of sr.
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

// ----------------------------------------------------------------------------
// TestResult

// TestResult captures the outcome of a single test run.
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

// ----------------------------------------------------------------------------
// CheckResult

// CheckResult captures the outcom of a single check inside a test.
type CheckResult struct {
	Name     string        // Name of the check as registered.
	JSON     string        // JSON serialization of check.
	Status   Status        // Outcome of check. All status but Error
	Duration time.Duration // How long the check took.
	Error    error         // For a Status of Bogus or Fail.
}

// ----------------------------------------------------------------------------
// Templates to output

var defaultCheckTmpl = `{{define "CHECK"}}{{printf "%-7s %-15s %s" .Status .Name .JSON}}` +
	`{{if eq .Status 3 5}} {{.Error.Error}}{{end}}{{end}}`

var htmlCheckTmpl = `{{define "CHECK"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}2">
  <div class="collapsed2">
    <h3 class="toggleButton2">Check 1:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▹
    </h3>
  </div>
  <div class="expanded2">
    <h3 class="toggleButton2">Check 1: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▾
    </h3>
    <div class="checkDetails">
      <div>Duration: {{.Duration}}</div>
      <div><code>{{.JSON}}</code></div>
      {{if eq .Status 3 5}}<div>{{.Error.Error}}</div>{{end}}
    </div>
  </div>
</div>
{{end}}
`

var defaultTestTmpl = `{{define "TEST"}}{{ToUpper .Status.String}}: {{.Name}}{{if gt .Tries 1}}
  {{printf "(after %d tries)" .Tries}}{{end}}
  Started: {{.Started}}   Duration: {{.FullDuration}}   Request: {{.Duration}}{{if .Error}}
  Error: {{.Error}}{{end}}
{{if eq .Status 2 3 4 5}}
  {{if .CheckResults}}Checks:
{{range $i, $c := .CheckResults}}{{printf "    %2d. " $i}}{{template "CHECK" .}}
{{end}}{{end}}{{end}}{{end}}`

var htmlTestTmpl = `{{define "TEST"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}">
  <div class="collapsed">
    <h2 class="toggleButton">Test 1:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "<code>{{.Name}}</code>"
      ({{.FullDuration}}) ▹
    </h2>
  </div>
  <div class="expanded">
    <h2 class="toggleButton">Test 1: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "<code>{{.Name}}</code>"
      ({{.FullDuration}}) ▾
    </h2>
    <div class="testDetails">
      <div id="summary">
	Started: {{.Started}}<br/>
	Full Duration: {{.FullDuration}} <br/>
        Number of tries: {{.Tries}} <br/>
        Request Duration: {{.Duration}}
        {{if .Error}}</br>Error: {{.Error}}{{end}}
      </div>
      {{if eq .Status 2 3 4 5}}{{if .CheckResults}}
        <div class="checks">
          {{range $i, $c := .CheckResults}}{{template "CHECK" .}}{{end}}
        </div>
      {{end}}{{end}}
      {{if .Response}}{{template "RESPONSE" .Response}}{{end}}
    </div>
  </div>
</div>
{{end}}`

var htmlResponseTmpl = `{{define "RESPONSE"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">HTTP Response ▾</h3>
    <div class="responseDetails">
      {{if .Response}}
        {{range $h, $v := .Response.Header}}
          {{range $v}}
            <code>{{printf "%s: %s" $h .}}</code></br>
          {{end}}
        {{end}}
      {{end}}
      {{if .BodyErr}}Error reading body: {{.BodyErr.Error}}
      {{else}} Body would go here
      {{end}}
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">HTTP Response ▹</h3>
  </div>
</div>
{{end}}
`

var defaultSuiteTmpl = `{{Box (printf "%s: %s" (ToUpper .Status.String) .Name) ""}}{{if .Error}}
Error: {{.Error}}{{end}}
Started: {{.Started}}   Duration: {{.FullDuration}}
Individual tests:
{{range .TestResults}}{{template "TEST" .}}{{end}}
`

var htmlSuiteTmpl = `<!DOCTYPE html>
<html>
<head>
<style>
.toggleButton { cursor: pointer; }
.toggleButton2 { cursor: pointer; }

.toggle .collapsed { display: block; }
.toggle .expanded { display: none; }
.toggleVisible .collapsed { display: none; }
.toggleVisible .expanded { display: block; }

.toggle2 .collapsed2 { display: block; }
.toggle2 .expanded2 { display: none; }
.toggleVisible2 .collapsed2 { display: none; }
.toggleVisible2 .expanded2 { display: block; }

h2 { 
  margin-top: 0.5em;
  margin-bottom: 0.2em;
}

h3 { 
  font-size: 1em;
  margin-top: 0.5em;
  margin-bottom: 0em;
}
.testDetails { margin-left: 1em; }
.checkDetails { margin-left: 2em; }
.requestDetails { margin-left: 2em; }
.responseDetails { margin-left: 2em; }

.PASS { color: green; }
.FAIL { color: red; }
.ERROR { color: magenta; }
.NOTRUN { color: grey; }
</style>

  <title>Suite {{.Name}}</title>


</head>
</body>
<h1>Results of Suite <code>{{.Name}}</code></h1>

{{.Description}}

<div id="summary">
  Status: <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> <br/>
  Started: {{.Started}} <br/>
  Full Duration: {{.FullDuration}}
</div>

{{range .TestResults}}{{template "TEST" .}}{{end}}

<script type="text/javascript" src="https://ajax.googleapis.com/ajax/libs/jquery/1.8.2/jquery.min.js"></script>
<script type="text/javascript">
(function() {
'use strict';

function bindToggle(el) {
  $('.toggleButton', el).click(function() {
    if ($(el).is('.toggle')) {
      $(el).addClass('toggleVisible').removeClass('toggle');
    } else {
      $(el).addClass('toggle').removeClass('toggleVisible');
    }
  });
}
function bindToggles(selector) {
  $(selector).each(function(i, el) {
    bindToggle(el);
  });
}

function bindToggle2(el) {
  console.log("bindToggle2 for " + el);
  $('.toggleButton2', el).click(function() {
    if ($(el).is('.toggle2')) {
      $(el).addClass('toggleVisible2').removeClass('toggle2');
    } else {
      $(el).addClass('toggle2').removeClass('toggleVisible2');
    }
  });
}

function bindToggles2(selector) {
console.log("bindToggles2("+selector+")");
  $(selector).each(function(i, el) {
    bindToggle2(el);
  });
}

$(document).ready(function() {
console.log("bindingstuff");
  bindToggles(".toggle");
  bindToggles(".toggleVisible");
  bindToggles2(".toggle2");
  bindToggles2(".toggleVisible2");
});

})();
</script>
</body>
</html>
`

var (
	TestTmpl      *template.Template
	SuiteTmpl     *template.Template
	HtmlSuiteTmpl *htmltemplate.Template
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

	HtmlSuiteTmpl = htmltemplate.New("SUITE")
	HtmlSuiteTmpl.Funcs(htmltemplate.FuncMap{"ToUpper": ToUpper})
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlSuiteTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlTestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlCheckTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlResponseTmpl))
}

func (r TestResult) PrintReport(w io.Writer) error {
	return TestTmpl.Execute(os.Stdout, r)
}

func (r SuiteResult) PrintReport(w io.Writer) error {
	return SuiteTmpl.Execute(os.Stdout, r)
}

func (r SuiteResult) HTMLReport(w io.Writer) error {
	return HtmlSuiteTmpl.Execute(w, r)
}
