// Copyright 2014 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ht

import (
	"fmt"
	htmltemplate "html/template"
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

// MarshalText implements encoding.TextMarshaler.
func (s Status) MarshalText() ([]byte, error) {
	if s < 0 || s > Bogus {
		return []byte(""), fmt.Errorf("no such status %d", s)
	}
	return []byte(s.String()), nil
}

// ----------------------------------------------------------------------------
// SuiteResult

// ----------------------------------------------------------------------------
// Templates to output

var defaultCheckTmpl = `{{define "CHECK"}}{{printf "%-7s %-15s %s" .Status .Name .JSON}}` +
	`{{if eq .Status 3 5}}{{range .Error}}
                {{.Error}}{{end}}{{end}}{{end}}`

var htmlCheckTmpl = `{{define "CHECK"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}2">
  <div class="collapsed2">
    <h3 class="toggleButton2">Check:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▹
    </h3>
  </div>
  <div class="expanded2">
    <h3 class="toggleButton2">Check: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span>
      <code>{{.Name}}</code> ▾
    </h3>
    <div class="checkDetails">
      <div>Checking took {{.Duration}}</div>
      <div><code>{{.JSON}}</code></div>
      {{if eq .Status 3 5}}<pre class="description">{{.Error.Error}}</pre>{{end}}
    </div>
  </div>
</div>
{{end}}
`

var defaultTestTmpl = `{{define "TEST"}}{{ToUpper .Status.String}}: {{.Name}}{{if gt .Tries 1}}
  {{printf "(after %d tries)" .Tries}}{{end}}
  Started: {{.Started}}   Duration: {{.FullDuration}}   Request: {{.Duration}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  Error: {{.Error}}{{end}}
{{if eq .Status 2 3 4 5}}  {{if .CheckResults}}Checks:
{{range $i, $c := .CheckResults}}{{printf "    %2d. " $i}}{{template "CHECK" .}}
{{end}}{{end}}{{end}}{{if .VarValues}}  Variables:
{{range $k, $v := .VarValues}}{{printf "    %s == %q\n" $k $v}}{{end}}{{end}}{{if .ExValues}}  Extracted:
{{range $k, $v := .ExValues}}{{if $v.Error}}{{printf "    %s : %s\n" $k $v.Error}}{{else}}{{printf "    %s == %q\n" $k $v.Value}}{{end}}{{end}}{{end}}{{end}}`

var shortTestTmpl = `{{define "SHORTTEST"}}{{.Status.String}}: {{.Name}}{{if .Request.Request}}
  {{.Request.Request.Method}} {{.Request.Request.URL.String}}{{range .Response.Redirections}}
  GET {{.}}{{end}}{{end}}{{if .Response.Response}}
  {{.Response.Response.Proto}} {{.Response.Response.Status}}{{end}}{{if .Error}}
  {{.Error}}{{end}}{{if gt .Status 2}}{{if .CheckResults}}{{range .CheckResults}}{{if eq .Status 3 5}}
    {{printf "%-7s %-15s %s" .Status .Name .JSON}}{{range .Error}}
      {{.Error}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .ExValues}}{{range $k, $v := .ExValues}}{{if $v.Error}}
  {{printf "Extraction of %s : %s\n" $k $v.Error}}{{end}}{{end}}{{end}}
{{end}}`

var htmlTestTmpl = `{{define "TEST"}}
<div class="toggle{{if gt .Status 2}}Visible{{end}}">
  <div class="collapsed">
    <h2 class="toggleButton">{{.Reporting.SeqNo}}:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{.Reporting.Filename}}</code>, {{.FullDuration}})</small> ▹
    </h2>
  </div>
  <div class="expanded">
    <h2 class="toggleButton">{{.Reporting.SeqNo}}: 
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{.Reporting.Filename}}</code>, {{.FullDuration}})</small> ▾
    </h2>
    <div class="testDetails">
      <div class="reqresp"><code>
        {{if .Request.Request}}
          <strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}<br/>
          {{range .Response.Redirections}}
            <strong>GET</strong> {{.}}<br/>
          {{end}}
        {{end}}
        {{if .Response.Response}}
          {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong>
        {{end}}
      </code></div>
      <div>
        {{if .Request.Request}}{{template "REQUEST" .}}{{end}}
        {{if .Response.Response}}{{template "RESPONSE" .}}{{end}}
        {{if .Request.SentParams}}{{template "FORMDATA" .Request.SentParams}}{{end}}
        <br/>
      </div>
      <div class="summary">
        <pre class="description">{{.Description}}</pre>
	Started: {{.Started}}<br/>
	Full Duration: {{.FullDuration}} <br/>
        Number of tries: {{.Tries}} <br/>
        Request Duration: {{.Duration}} <br/>
        {{if .VarValues}}Variables:<br/>
          {{range $k, $v := .VarValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .ExValues}}Extractions:<br/>
          {{range $k, $v := .ExValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .Error}}<br/><strong>Error:</strong> {{.Error}}{{end}}<br/>
      </div>
      {{if eq .Status 2 3 4 5}}{{if .CheckResults}}
        <div class="checks">
          {{range $i, $c := .CheckResults}}{{template "CHECK" .}}{{end}}
        </div>
      {{end}}{{end}}
    </div>
  </div>
</div>
{{end}}`

var htmlHeaderTmpl = `{{define "HEADER"}}
<div class="httpheader">
  {{range $h, $v := .}}
    {{range $v}}
      <code><strong>{{printf "%25s: " $h}}</strong> {{.}}</code></br>
    {{end}}
  {{end}}
</div>
{{end}}`

var htmlResponseTmpl = `{{define "RESPONSE"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">HTTP Response ▾</h3>
    <div class="responseDetails">
      {{if .Response.Response}}
        {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong><br/>
        {{template "HEADER" .Response.Response.Header}}
      {{end}}
      {{if .Response.BodyErr}}Error reading body: {{.Response.BodyErr.Error}}
      {{else}}
<pre class="responseBodySummary">
{{Summary .Response.BodyStr}}
</pre>
        <a href="{{.Reporting.SeqNo}}.ResponseBody.{{.Reporting.Extension}}" target="_blank">Response Body</a>
      {{end}}
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">HTTP Response ▹</h3>
  </div>
</div>
{{end}}
`

var htmlRequestTmpl = `{{define "REQUEST"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">HTTP Request ▾</h3>
    <div class="requestDetails">
      <code><strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}
          {{range .Response.Redirections}}</br>GET {{.}}{{end}}
      </code>
      {{template "HEADER" .Request.Request.Header}}
<pre>{{.Request.SentBody}}</pre>
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">HTTP Request ▹</h3>
  </div>
</div>
{{end}}
`

var htmlFormdataTmpl = `{{define "FORMDATA"}}
<div class="toggle2">
  <div class="expanded2">
    <h3 class="toggleButton2">Form Data ▾</h3>
    <div class="formdataDetails">
      {{range $k, $vs := .}}
        {{range $v := $vs}}
          <code><strong>{{printf "%25s: " $k}}</strong>{{printf "%q" $v}}</code></br>
        {{end}}
      {{end}}
    </div>
  </div>
  <div class="collapsed2">
    <h3 class="toggleButton2">Form Data ▹</h3>
  </div>
</div>
{{end}}
`

var (
	ShortTestTmpl  *template.Template
	TestTmpl       *template.Template
	SuiteTmpl      *template.Template
	ShortSuiteTmpl *template.Template
	HtmlSuiteTmpl  *htmltemplate.Template
)

func init() {
	fm := make(template.FuncMap)
	fm["Underline"] = Underline
	fm["Box"] = Box
	fm["ToUpper"] = strings.ToUpper

	ShortTestTmpl = template.New("SHORTTEST")
	ShortTestTmpl.Funcs(fm)
	ShortTestTmpl = template.Must(ShortTestTmpl.Parse(shortTestTmpl))

	TestTmpl = template.New("TEST")
	TestTmpl.Funcs(fm)
	TestTmpl = template.Must(TestTmpl.Parse(defaultTestTmpl))
	TestTmpl = template.Must(TestTmpl.Parse(defaultCheckTmpl))

}

// PrintReport of t to w.
func (t Test) PrintReport(w io.Writer) error {
	return TestTmpl.Execute(w, t)
}

// PrintShortReport of t to w.
func (t Test) PrintShortReport(w io.Writer) error {
	return ShortTestTmpl.Execute(w, t)
}
