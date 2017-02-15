// Copyright 2016 Volker Dobler.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package suite

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"io/ioutil"
	"mime"
	"os"
	"path"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/vdobler/ht/ht"
	mimelist "github.com/vdobler/ht/mime"
)

// ----------------------------------------------------------------------------
// Templates to output

var htmlCheckTmpl = `{{define "CHECK"}}
<div class="toggle2{{if gt .Check.Status 2}}Visible{{end}}2">
  <input type="checkbox" value="selected" {{if gt .Check.Status 2}}checked{{end}}
         id="check-{{.SeqNo}}-{{.N}}" class="toggle-input">
  <label for="check-{{.SeqNo}}-{{.N}}" class="toggle-label">
    <h3><span class="{{ToUpper .Check.Status.String}}">{{ToUpper .Check.Status.String}}</span>
      <code>{{.Check.Name}}</code></h3>
  </label>
  <div class="toggle-content">
    <div class="checkDetails">
      <div>Checking took {{niceduration .Check.Duration}}</div>
      <div><code>{{.Check.JSON}}</code></div>
      {{if eq .Check.Status 3 5}}<pre class="description">{{.Check.Error.Error}}</pre>{{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlTestTmpl = `{{define "TEST"}}
<div class="toggle">
  <input type="checkbox" value="selected" {{if gt .Status 2}}checked{{end}}
         id="test-{{.Reporting.SeqNo}}" class="toggle-input">
  <label for="test-{{.Reporting.SeqNo}}" class="toggle-label">
    <h2>{{.Reporting.SeqNo}}:
      <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> 
      "{{.Name}}" <small>(<code>{{.Reporting.Filename}}</code>, {{niceduration .FullDuration}})</small>
    </h2>
  </label>
  <div class="toggle-content">
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
      <div class="summary">
        <pre class="description">{{.Description}}</pre>
	Started: {{nicetime .Started}}<br/>
	Full Duration: {{niceduration .FullDuration}} <br/>
        Number of tries: {{.Tries}} <br/>
        Request Duration: {{niceduration .Duration}} <br/>
        {{if .Error}}<br/><strong>Error:</strong> {{.Error}}<br/>{{end}}
      </div>
      {{if .Request.Request}}{{template "REQUEST" .}}{{end}}
      {{if .Response.Response}}{{template "RESPONSE" .}}{{end}}
      {{if .Request.SentParams}}{{template "FORMDATA" dict "Params" .Request.SentParams "SeqNo" .Reporting.SeqNo}}{{end}}
      {{if or .Variables .ExValues}}{{template "VARIABLES" .}}{{end}}
      {{if eq .Status 2 3 4 5}}{{if .CheckResults}}
        <div class="checks">
          {{range $i, $e := .CheckResults}}
{{template "CHECK" dict "Check" . "SeqNo" $.Reporting.SeqNo "N" $i}}
          {{end}}
        </div>
      {{end}}{{end}}
      <div>
        <div class="toggle">
          <input type="checkbox" value="selected"
                 id="curl-{{.Reporting.SeqNo}}" class="toggle-input">
          <label for="curl-{{.Reporting.SeqNo}}" class="toggle-label"><h3>Curl Call</h3></label>
          <div class="toggle-content">
            <div>
<pre>
{{.CurlCall}}
</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</div>
{{end}}`

var htmlHeaderTmpl = `{{define "HEADER"}}
<div class="httpheader">
  {{range $h, $v := .}}
    {{range $v}}
      <code><strong>{{printf "%25s: " $h}}</strong> {{.}}</code><br>
    {{end}}
  {{end}}
</div>
{{end}}`

var htmlResponseTmpl = `{{define "RESPONSE"}}
<div class="toggle">
  <input type="checkbox" value="selected" checked
         id="resp-{{.Reporting.SeqNo}}" class="toggle-input">
  <label for="resp-{{.Reporting.SeqNo}}" class="toggle-label"><h3>HTTP Response</h3></label>
  <div class="toggle-content">
    <div class="responseDetails">
      {{if .Response.Response}}
        {{.Response.Response.Proto}} <strong>{{.Response.Response.Status}}</strong><br/>
        {{template "HEADER" .Response.Response.Header}}
      {{end}}
      {{if .Response.BodyErr}}Error reading body: {{.Response.BodyErr.Error}}
      {{else}}
        {{if .Response.BodyStr}}
<pre class="responseBodySummary">
{{Summary .Response.BodyStr}}
</pre>
        <a href="{{.Reporting.SeqNo}}.ResponseBody.{{.Reporting.Extension}}" target="_blank">Response Body</a>
        {{else}}
          &#x2014; &#x2003; no body &#x2003; &#x2014;
        {{end}}
      {{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlRequestTmpl = `{{define "REQUEST"}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="req-{{.Reporting.SeqNo}}" class="toggle-input">
  <label for="req-{{.Reporting.SeqNo}}" class="toggle-label"><h3>HTTP Request</h3></label>
  <div class="toggle-content">
    <div class="requestDetails">
      <code><strong>{{.Request.Request.Method}}</strong> {{.Request.Request.URL.String}}
          {{range .Response.Redirections}}<br>GET {{.}}{{end}}
      </code>
      {{template "HEADER" .Request.Request.Header}}
<pre>{{clean .Request.SentBody}}</pre>
    </div>
  </div>
</div>
{{end}}
`

var htmlFormdataTmpl = `{{define "FORMDATA"}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="form-{{.SeqNo}}" class="toggle-input">
  <label for="form-{{.SeqNo}}" class="toggle-label"><h3>Form Data</h3></label>
  <div class="toggle-content">
    <div class="formdataDetails">
      {{range $k, $vs := .Params}}
        {{range $v := $vs}}
          <code><strong>{{printf "%25s: " $k}}</strong>{{printf "%q" $v}}</code><br>
        {{end}}
      {{end}}
    </div>
  </div>
</div>
{{end}}
`

var htmlVariablesTmpl = `{{define "VARIABLES"}}
<div class="toggle">
  <input type="checkbox" value="selected"
         id="var-{{.Reporting.SeqNo}}" class="toggle-input">
  <label for="var-{{.Reporting.SeqNo}}" class="toggle-label"><h3>Variables</h3></label>

  <div class="toggle-content">
    <div class="variableDetail">
        {{if .Variables}}Variables:<br/>
          {{range $k, $v := .Variables}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
        {{if .ExValues}}Extractions:<br/>
          {{range $k, $v := .ExValues}}
            <code>&nbsp;&nbsp;{{printf "%s = %q" $k $v}}</code><br/>
          {{end}}
        {{end}}
    </div>
  </div>
</div>
{{end}}
`

var defaultSuiteTmpl = `{{Box (printf "%s: %s" (ToUpper .Status.String) .Name) ""}}{{if .Error}}
Error: {{.Error}}{{end}}
Started: {{.Started}}   Duration: {{niceduration .Duration}}

{{range .Tests}}{{template "TEST" .}}
{{end}}
`

var shortSuiteTmpl = `======  Result of {{.Name}} =======
{{range .Tests}}{{template "SHORTTEST" .}}{{end}}{{printf "===> %s <=== %s" (ToUpper .Status.String) .Name}}
`

var htmlStyleTmpl = `{{define "STYLE"}}
<style>

.toggle {
	margin: 0 auto;
        padding-top: 0.2em;
        padding-bottom: 0.2em;
}

.toggle-label {
	cursor: pointer;
	display: block;
}

.toggle-label:after {
	content: " ▾";
}

.toggle-content {
	margin-bottom: 0.5ex;
}

.toggle-input {
	display: none;
}

.toggle-input:not(checked) ~ .toggle-content {
	display: none;
}

.toggle-input:checked ~ .toggle-content {
	display: block;
}

.toggle-input:checked ~ .toggle-label:after {
	content: " ▹";
}

.summary {
  padding: 1ex 0 1ex 0;
}

.checks {
  padding: 1ex 0 1ex 0;
}

h2 { 
  margin-top: 0.5em;
  margin-bottom: 0.2em;
  display: inline;
}

h3 { 
  font-size: 1em;
  margin-top: 0.5em;
  margin-bottom: 0em;
  display: inline;
}
.testDetails { margin-left: 1em; }
.checkDetails { margin-left: 2em; }
.requestDetails { margin-left: 2em; }
.responseDetails { margin-left: 2em; }
.formdataDetails { margin-left: 2em; }

.PASS { color: green; }
.FAIL { color: red; }
.ERROR { color: magenta; }
.NOTRUN { color: grey; }

pre.description { font-family: serif; margin: 0px; }
</style>
{{end}}`

var htmlSuiteTmpl = `<!DOCTYPE html>
<html>
<head>
  <meta http-equiv="content-type" content="text/html; charset=UTF-8" />
  {{template "STYLE"}}
  <title>Suite {{.Name}}</title>
</head>
<body>
<a href="../">Up/Back/Home</a>


<h1>Results of Suite "{{.Name}}"</h1>

{{.Description}}

<div class="summary">
  Status: <span class="{{ToUpper .Status.String}}">{{ToUpper .Status.String}}</span> <br/>
  Started: {{.Started}} <br/>
  Full Duration: {{niceduration .Duration}}
</div>

{{range .Tests}}{{template "TEST" .}}{{end}}

</body>
</html>
`

// Templates used to generate default and short text output and HTML page.
var (
	SuiteTmpl      *template.Template
	ShortSuiteTmpl *template.Template
	HtmlSuiteTmpl  *htmltemplate.Template
)

// LoopIteration helps ranging over Data in a template.
type LoopIteration struct {
	Data      interface{}
	I         int // 0-based loop index
	N         int // 1-based loop index
	Odd, Even bool
}

func loopIteration(idx interface{}, data interface{}) (LoopIteration, error) {
	i, ok := idx.(int)
	if !ok {
		return LoopIteration{}, fmt.Errorf("idx is not int")
	}
	return LoopIteration{
		Data: data,
		I:    i,
		N:    i + 1,
		Odd:  i%2 == 0,
		Even: i%2 == 1,
	}, nil
}

func dict(args ...interface{}) (map[string]interface{}, error) {
	n := len(args)
	if n%2 == 1 {
		return nil, errors.New("odd number of arguments to dict")
	}
	dict := make(map[string]interface{}, n/2)
	for i := 0; i < n; i += 2 {
		key, ok := args[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict key of type %T", args[i])
		}
		dict[key] = args[i+1]
	}
	return dict, nil
}

func cleanSentBody(s string) string {
	runes := make([]rune, 0, 2000)
	for i, r := range s {
		if r < 0x1f || (r >= 0x7f && r <= 0x9f) {
			runes = append(runes, '\uFFFD')
		} else {
			runes = append(runes, r)
		}
		if i >= 4090 {
			runes = append(runes, '\u2026')
			break
		}
	}
	return string(runes)
}

func roundTimeToMS(t time.Time) time.Time {
	return t.Round(time.Millisecond)
}

// roundDuration d to approximately 3 significant digits (but not less than
// to full second).
func roundDuration(d time.Duration) time.Duration {
	round := func(d time.Duration, to time.Duration) time.Duration {
		return to * ((d + to/2) / to)
	}
	min, sec, ms, mu, ns := time.Minute, time.Second, time.Millisecond, time.Microsecond, time.Nanosecond

	// TODO: refactor once loops are invented.
	if d >= 1*min {
		return round(d, sec)
	} else if d >= 10*sec {
		return round(d, 100*ms)
	} else if d >= 1*sec {
		return round(d, 10*ms)
	} else if d >= 100*ms {
		return round(d, 1*ms)
	} else if d >= 10*ms {
		return round(d, 100*mu)
	} else if d >= 1*ms {
		return round(d, 10*mu)
	} else if d >= 1*ms {
		return round(d, 10*mu)
	} else if d >= 100*mu {
		return round(d, 1*mu)
	} else if d >= 10*mu {
		return round(d, 100*ns)
	} else if d >= 1*mu {
		return round(d, 100*ns)
	} else if d >= 100*ns {
		return round(d, 10*ns)
	}
	return d
}

func init() {
	fm := make(template.FuncMap)
	//fm["Underline"] = Underline
	fm["Box"] = ht.Box
	fm["ToUpper"] = strings.ToUpper
	fm["nicetime"] = roundTimeToMS
	fm["niceduration"] = roundDuration

	SuiteTmpl = template.New("SUITE")
	SuiteTmpl.Funcs(fm)
	SuiteTmpl = template.Must(SuiteTmpl.Parse(defaultSuiteTmpl))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(ht.DefaultTestTemplate))
	SuiteTmpl = template.Must(SuiteTmpl.Parse(ht.DefaultCheckTemplate))

	ShortSuiteTmpl = template.New("SHORTSUITE")
	ShortSuiteTmpl.Funcs(fm)
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(shortSuiteTmpl))
	ShortSuiteTmpl = template.Must(ShortSuiteTmpl.Parse(ht.ShortTestTemplate))

	HtmlSuiteTmpl = htmltemplate.New("SUITE")
	HtmlSuiteTmpl.Funcs(htmltemplate.FuncMap{
		"ToUpper":      strings.ToUpper,
		"Summary":      ht.Summary,
		"loop":         loopIteration,
		"dict":         dict,
		"clean":        cleanSentBody,
		"nicetime":     roundTimeToMS,
		"niceduration": roundDuration,
	})
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlSuiteTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlTestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlCheckTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlResponseTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlRequestTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlHeaderTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlFormdataTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlVariablesTmpl))
	HtmlSuiteTmpl = htmltemplate.Must(HtmlSuiteTmpl.Parse(htmlStyleTmpl))
}

// PrintReport outputs a textual report of s to w.
func (s *Suite) PrintReport(w io.Writer) error {
	return SuiteTmpl.Execute(w, s)
}

// PrintShortReport outputs a short textual report of s to w.
func (s *Suite) PrintShortReport(w io.Writer) error {
	return ShortSuiteTmpl.Execute(w, s)
}

// TODO: sniff if unavailable
func guessResponseExtension(test *ht.Test) string {
	if test.Response.Response == nil || len(test.Response.BodyStr) == 0 {
		return "nil"
	}

	ct := test.Response.Response.Header.Get("Content-Type")
	if ct == "" {
		return "txt"
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return "txt"
	}

	if ext, ok := mimelist.MimeTypeExtension[mt]; ok {
		return ext
	}

	return "txt"
}

// HTMLReport generates a report of the outcome of s to directory dir.
func HTMLReport(dir string, s *Suite) error {
	errs := ht.ErrorList{}

	for _, test := range s.Tests {
		if tn, ok := test.Variables["TEST_NAME"]; ok {
			test.Reporting.Filename = tn
		} else {
			test.Reporting.Filename = "??"
		}
		test.Reporting.Extension = guessResponseExtension(test)

		body := []byte(test.Response.BodyStr)
		fn := fmt.Sprintf("%s.ResponseBody.%s", test.Reporting.SeqNo,
			test.Reporting.Extension)
		name := path.Join(dir, fn)
		err := ioutil.WriteFile(name, body, 0666)
		if err != nil {
			errs = append(errs, err)
		}
	}

	report, err := os.Create(path.Join(dir, "_Report_.html"))
	if err != nil {
		errs = append(errs, err)
	} else {
		err = HtmlSuiteTmpl.Execute(report, s)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errs
}

// JUnit style output.
// ----------------------------------------------------------------------------

// JUnit4XML generates a JUnit 4 compatible XML result with each Check
// reported as an individual testcase.
// NotRun checks are reported as Skipped and Bogus checks are counted as
// Errored tests.
func (s *Suite) JUnit4XML() (string, error) {
	// Local types used for XML encoding
	type SysOut struct {
		XMLName xml.Name `xml:"system-out"`
		Data    string   `xml:",innerxml"`
	}
	type ErrorMsg struct {
		Message string `xml:"message,attr"`
		Typ     string `xml:"type,attr"`
	}
	type Testcase struct {
		XMLName   xml.Name  `xml:"testcase"`
		Name      string    `xml:"name,attr"`
		Classname string    `xml:"classname,attr"`
		Time      float64   `xml:"time,attr"`
		Skipped   *struct{} `xml:"Skipped,omitempty"`
		Error     *ErrorMsg `xml:"error,omitempty"`
		Failure   *ErrorMsg `xml:"failure,omitempty"`
		SystemOut string    `xml:"system-out,omitempty"`
	}
	type Property struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}
	type Testsuite struct {
		XMLName xml.Name `xml:"testsuite"`
		Tests   int      `xml:"tests,attr"`
		// Disabled   int        `xml:"disabled,attr"`
		Errors     int        `xml:"errors,attr"`
		Failures   int        `xml:"failures,attr"`
		Skipped    int        `xml:"skipped,attr"`
		Time       float64    `xml:"time,attr"`
		Timestamp  string     `xml:"timestamp,attr"`
		Properties []Property `xml:"properties>property"`
		Testcase   []Testcase
		SystemOut  SysOut
	}

	// Unwind all Checks to their own testcase.
	skipped, passed, failed, errored := 0, 0, 0, 0
	testcases := []Testcase{}
	for _, test := range s.Tests {
		if test.Status >= ht.Error {
			// report all checks as errored but with special message
			for _, cr := range test.CheckResults {
				tc := Testcase{
					Name:      cr.Name,
					Classname: test.Name,
					SystemOut: cr.JSON,
				}
				tc.Error = &ErrorMsg{
					Message: test.Error.Error(),
					Typ:     fmt.Sprintf("main test error, check not run"),
				}
				errored++
				testcases = append(testcases, tc)
			}

		} else {
			for _, cr := range test.CheckResults {
				tc := Testcase{
					Name:      cr.Name,
					Classname: test.Name,
					Time:      float64(cr.Duration) / 1e9,
					SystemOut: cr.JSON,
				}

				switch cr.Status {
				case ht.NotRun, ht.Skipped:
					tc.Skipped = &struct{}{}
					skipped++
				case ht.Pass:
					passed++
				case ht.Fail:
					tc.Failure = &ErrorMsg{
						Message: cr.Error.Error(),
						Typ:     fmt.Sprintf("%T", test.Error),
					}
					failed++
				case ht.Error, ht.Bogus:
					tc.Error = &ErrorMsg{
						Message: test.Error.Error(),
						Typ:     fmt.Sprintf("%T", test.Error),
					}
					errored++
				default:
					panic(cr.Status)
				}

				testcases = append(testcases, tc)
			}
		}
	}

	// Generate a standard text report which becomes the standard-out of
	// the generated JUnit report.
	buf := &bytes.Buffer{}
	var sysout string
	err := s.PrintReport(buf)
	if err != nil {
		sysout = err.Error()
	} else {
		sysout = xmlEscapeChars(buf.Bytes())
	}

	// Populate the Testsuite type for marshalling.
	ts := Testsuite{
		Tests:     skipped + passed + failed + errored,
		Errors:    errored,
		Failures:  failed,
		Skipped:   skipped,
		Time:      float64(s.Duration) / 1e9,
		Timestamp: s.Started.Format("2006-01-02T15:04:05"),
		Testcase:  testcases,
		SystemOut: SysOut{Data: "\n" + sysout},
	}
	for k, v := range s.Variables {
		ts.Properties = append(ts.Properties, Property{Name: k, Value: v})
	}

	data, err := xml.MarshalIndent(ts, "", "  ")
	if err != nil {
		return string(data), err
	}
	return xml.Header + string(data) + "\n", nil
}

// xmlEscapeChars escapes the reserved characters. TODO: \r ?
func xmlEscapeChars(s []byte) string {
	buf := &bytes.Buffer{}
	for i := 0; i < len(s); {
		rune, width := utf8.DecodeRune(s[i:])
		i += width
		switch rune {
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '&':
			buf.WriteString("&amp;")
		case '"':
			buf.WriteString("&quot;")
		case '\'':
			buf.WriteString("&apos;")
		case '\t':
			buf.WriteString("&#x9;")
		default:
			// TODO: not every rune is allowed in XML
			buf.WriteRune(rune)
		}
	}
	return buf.String()
}
